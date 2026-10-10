package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Asking for help must not start the interface.
//
// This is the whole point of the flag: today `toki --help` reads no arguments at
// all, goes straight to launching Bubble Tea, and dies with "could not open TTY".
// So a test that runs it with no terminal attached and expects clean output is a
// test that fails against the old behaviour rather than merely describing the new.
func TestAskingForHelpDoesNotStartTheInterface(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--help) = %v, want nil", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Usage") {
		t.Errorf("--help printed no usage:\n%s", out)
	}
	if stderr.Len() != 0 {
		t.Errorf("--help wrote to stderr: %s", stderr.String())
	}
}

// Help goes to stdout so it can be piped into less, which is how everyone reads a
// long usage text.
func TestHelpGoesToStdoutSoItCanBePiped(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--help) = %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("--help wrote nothing to stdout")
	}
	if strings.Contains(stdout.String(), "Usage") && stderr.Len() != 0 {
		t.Errorf("--help split itself across both streams: stdout %q, stderr %q",
			stdout.String(), stderr.String())
	}
}

// -h is the spelling everyone actually types.
func TestTheShortHelpFlagWorksToo(t *testing.T) {
	var long, short, ignored bytes.Buffer

	if err := run([]string{"--help"}, &long, &ignored); err != nil {
		t.Fatalf("run(--help) = %v", err)
	}
	if err := run([]string{"-h"}, &short, &ignored); err != nil {
		t.Fatalf("run(-h) = %v", err)
	}

	if long.String() != short.String() {
		t.Error("-h and --help print different things")
	}
}

// Listing shows the splits from the config without launching anything, which is the
// question "did my config edit work?" asked from a shell.
func TestListingTheSplitsDoesNotStartTheInterface(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n"+
		"    cycles: 4\n    long_break: 15\n"+
		"  - name: Long\n    focus: 50\n    break: 10\n"+
		"    cycles: 3\n    long_break: 20\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--list"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--list) = %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"Classic", "25", "Long", "50"} {
		if !strings.Contains(out, want) {
			t.Errorf("--list is missing %q:\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("--list wrote to stderr: %s", stderr.String())
	}
}

// The listing has to report the durations as they will actually run, so the values
// the config left unset are filled in before printing.
func TestListingReportsTheDurationsThatWillActuallyRun(t *testing.T) {
	// No cycles or long_break: both are optional and default.
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--list"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--list) = %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "4") {
		t.Errorf("--list did not fill in the default round count:\n%s", out)
	}
	if !strings.Contains(out, "15") {
		t.Errorf("--list did not fill in the default long break:\n%s", out)
	}
}

// Listing a config that cannot be parsed is worth reporting, not swallowing.
func TestListingABrokenConfigSaysSoAndFails(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: [not a number\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--list"}, &stdout, &stderr); err == nil {
		t.Errorf("run(--list) = nil on a broken config, want an error:\n%s", stdout.String())
	}
}

func TestTheVersionIsPrintedWithoutStartingTheInterface(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := run([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--version) = %v, want nil", err)
	}

	if !strings.Contains(stdout.String(), version) {
		t.Errorf("--version printed %q, want it to contain %q", stdout.String(), version)
	}
	if stderr.Len() != 0 {
		t.Errorf("--version wrote to stderr: %s", stderr.String())
	}
}

// A typo must not launch a full screen interface; that is disorienting and it hides
// the mistake.
func TestAnUnknownFlagIsRejectedWithSomethingReadable(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"--focuss"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run(--focuss) = nil, want it rejected")
	}
	if !strings.Contains(err.Error(), "focuss") {
		t.Errorf("error %q does not name the flag that was wrong", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("a rejected flag still printed to stdout: %s", stdout.String())
	}
}

// Every flag the program answers to has to appear in its own help text, so a new
// flag cannot be added without anyone learning it exists.
func TestTheHelpTextMentionsEveryFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--help) = %v", err)
	}

	out := stdout.String()
	for _, flag := range []string{"--help", "--version", "--list"} {
		if !strings.Contains(out, flag) {
			t.Errorf("the help text never mentions %s:\n%s", flag, out)
		}
	}
}

// The help text is the only description of the keys that exists outside the running
// interface, so it has to carry them.
func TestTheHelpTextSaysHowToUseTheInterface(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--help) = %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"config.yaml", "pause", "skip"} {
		if !strings.Contains(out, want) {
			t.Errorf("the help text does not mention %q:\n%s", want, out)
		}
	}
}

// stubConfig points HOME at a fresh directory holding the given config, so a test
// never reads or writes the developer's real one.
func stubConfig(t *testing.T, body string) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), ".config/toki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Dir(filepath.Dir(dir)))
}

// A successful run says nothing at all on stderr.
//
// This is a regression test for a bug this file could not previously catch: every
// test here called run directly, so nothing checked what main did with a nil error,
// and a refactor had it print "toki: <nil>" on every successful invocation. Testing
// the body is not the same as testing the program's whole decision about output.
func TestASuccessfulRunSaysNothingOnStderr(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n")

	for _, args := range [][]string{{"--version"}, {"--list"}, {"--help"}} {
		var stdout, stderr bytes.Buffer

		if got := exit(args, &stdout, &stderr); got != 0 {
			t.Errorf("exit(%v) = %d, want 0", args, got)
		}
		if stderr.Len() != 0 {
			t.Errorf("exit(%v) wrote to stderr on success: %q", args, stderr.String())
		}
	}
}

// A failure has to say what went wrong and report a non-zero status, so a script
// calling toki can tell.
func TestAFailureIsReportedOnceAndExitsNonZero(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: [not a number\n")

	var stdout, stderr bytes.Buffer

	if got := exit([]string{"--list"}, &stdout, &stderr); got != 1 {
		t.Errorf("exit(--list) on a broken config = %d, want 1", got)
	}
	if got := strings.Count(stderr.String(), "toki:"); got != 1 {
		t.Errorf("the failure was reported %d times, want once:\n%s", got, stderr.String())
	}
}

// A mistyped flag is explained on the way through run, with the usage text
// attached. main must not then say it a second time.
func TestABadFlagIsNotReportedTwice(t *testing.T) {
	var stdout, stderr bytes.Buffer

	got := exit([]string{"--focuss"}, &stdout, &stderr)

	if got != 1 {
		t.Errorf("exit(--focuss) = %d, want 1", got)
	}
	if n := strings.Count(stderr.String(), "focuss"); n != 1 {
		t.Errorf("the typo is mentioned %d times, want once:\n%s", n, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage") {
		t.Errorf("a mistyped flag did not come with the usage text:\n%s", stderr.String())
	}
}

// The usage text is the only description of the flags that exists outside the
// running program, so a flag that does not appear in it is a flag nobody can
// discover.
func TestTheUsageTextNamesEveryFlag(t *testing.T) {
	var stdout bytes.Buffer
	if err := run([]string{"--help"}, &stdout, io.Discard); err != nil {
		t.Fatal(err)
	}

	for _, flag := range []string{"--stats", "--history", "--list", "--version"} {
		if !strings.Contains(stdout.String(), flag) {
			t.Errorf("the usage text does not mention %s:\n%s", flag, stdout.String())
		}
	}
}

// An unknown flag has to be refused rather than ignored, or a typo would quietly
// run an ordinary session while the user believes they asked for something.
func TestAnUnknownFlagIsRefused(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run([]string{"--stat"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run() = nil, want an error for an unknown flag")
	}
	if !strings.Contains(stderr.String(), "--stat") {
		t.Errorf("stderr does not name the offending flag:\n%s", stderr.String())
	}
}

// These are questions asked from a shell, so they are answered from a shell. A
// full screen interface would have to be drawn, redrawn and dismissed before the
// number could be read, and the output could not be piped.
func TestTheStatisticsArePrintedRatherThanDrawn(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n")

	var stdout, stderr bytes.Buffer

	if err := run([]string{"--stats"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(--stats) returned error: %v\n%s", err, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "today") {
		t.Errorf("--stats does not print today's total:\n%s", out)
	}
	if stderr.Len() != 0 {
		t.Errorf("--stats wrote to stderr:\n%s", stderr.String())
	}
}

// With no history there is no number to print, and printing a zero would read as
// a figure rather than an absence.
func TestTheStatisticsSaySoWhenNothingIsRecorded(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--stats"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "nothing recorded yet") {
		t.Errorf("--stats does not say the history is empty:\n%s", stdout.String())
	}
}

// Both flags print both sections, rather than one winning silently.
func TestBothStatisticsFlagsPrintBothSections(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--stats", "--history"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	out := stdout.String()
	if !strings.Contains(out, "today") {
		t.Errorf("--stats --history does not print today:\n%s", out)
	}
	if !strings.Contains(out, "last 3 months") {
		t.Errorf("--stats --history does not print the history:\n%s", out)
	}
}

// The window the history prints has to be the window retention keeps, or the
// output claims to show records that have already been pruned.
func TestTheHistoryNamesTheWindowItKeeps(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--history"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "last 3 months") {
		t.Errorf("the window is not named:\n%s", stdout.String())
	}
}

// The headings are what separate the two tables, so a report whose headings lost
// their styling still has to carry the text — and the styling has to be there for
// them to be readable as headings rather than as more output.
func TestTheStatisticsHeadingsAreStyled(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--stats"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	out := stdout.String()
	if !strings.Contains(out, "today") {
		t.Errorf("the heading text is missing:\n%q", out)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("the report is entirely unstyled:\n%q", out)
	}
}

// Every heading gets the same breathing room, the first one included. A report that
// opens on a heading in the very first column reads as though its top had been cut
// off, and one where the first heading is tighter than the ones after it looks
// assembled rather than laid out.
func TestEveryHeadingIsPrecededByABlankLine(t *testing.T) {
	stubConfig(t, "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n")

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--stats", "--history"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(stdout.String(), "\n")

	for i, line := range lines {
		if !strings.Contains(line, "today") && !strings.Contains(line, "last 3 months") {
			continue
		}

		// The first heading is preceded by the empty first line rather than by an
		// index of -1, so both cases are covered by the same check.
		if i == 0 || strings.TrimSpace(lines[i-1]) != "" {
			t.Errorf("the heading on line %d is not preceded by a blank line:\n%s", i, stdout.String())
		}
	}
}
