package history

import (
	"os"
	"strings"
	"testing"
	"time"
)

// now is the instant every retention test measures from, so that the records
// below can be written as plain offsets from it.
var now = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func recordAt(name string, offset time.Duration) Record {
	return Record{Name: name, Focused: 25 * time.Minute, Rounds: 4, EndedAt: now.Add(offset)}
}

// seed writes records to the history file, bypassing prune so that a test can
// build a history older than the retention window in the first place.
func seed(t *testing.T, records ...Record) string {
	t.Helper()

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(strings.TrimSuffix(path, File), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, r := range records {
		if err := Append(r); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// The whole point of retention: a record older than the window goes away, and
// one inside it stays.
func TestRecordsOlderThanTheWindowArePruned(t *testing.T) {
	home(t)
	path := seed(t,
		recordAt("ancient", -400*24*time.Hour),
		recordAt("recent", -time.Hour),
	)

	if err := prune(path, now); err != nil {
		t.Fatalf("prune() returned error: %v", err)
	}

	records, err := Since(now.AddDate(-100, 0, 0))
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Name != "recent" {
		t.Errorf("kept %q, want %q", records[0].Name, "recent")
	}
}

// A record sitting exactly on the boundary is kept, so that what --history shows
// and what is still on disk are the same window.
func TestARecordExactlyOnTheBoundaryIsKept(t *testing.T) {
	home(t)

	boundary := Record{
		Name:    "on-the-edge",
		Focused: 25 * time.Minute,
		Rounds:  4,
		EndedAt: now.AddDate(0, -RetentionMonths, 0),
	}
	path := seed(t, boundary)

	if err := prune(path, now); err != nil {
		t.Fatal(err)
	}

	records, err := Since(now.AddDate(-100, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Name != "on-the-edge" {
		t.Errorf("kept %q, want %q", records[0].Name, "on-the-edge")
	}
}

// The common case is a history that is entirely inside the window. Rewriting it
// would be pointless work on every single session.
func TestPruningLeavesAnInWindowHistoryUntouched(t *testing.T) {
	home(t)

	path := seed(t, recordAt("a", -2*time.Hour), recordAt("b", -time.Hour))

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := prune(path, now); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if !before.ModTime().Equal(after.ModTime()) {
		t.Errorf("the file was rewritten even though nothing had aged out")
	}
}

// Pruning rewrites the file, so it is the one moment a torn line can be cleaned
// up. Reading skips a torn line; the rewrite should not keep it forever.
func TestPruningDropsALineItCannotDecode(t *testing.T) {
	home(t)
	path := seed(t, recordAt("good", -time.Hour))

	// The mess a machine losing power mid-write leaves behind.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("{\"name\":\"half-writ"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	// Force a rewrite by making the window require it.
	if err := prune(path, now.AddDate(0, -1, 0)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), "half-writ") {
		t.Errorf("the torn line survived the rewrite:\n%s", data)
	}
}

// Records that all prune away leave an empty file rather than a missing one. The
// difference matters: the file existing is what tells a later run there is
// nothing to show, without it having to distinguish empty from absent.
func TestPruningEverythingLeavesAnEmptyFile(t *testing.T) {
	home(t)
	path := seed(t, recordAt("ancient", -400*24*time.Hour))

	if err := prune(path, now); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file should still exist: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("got %d bytes, want an empty file", len(data))
	}
}

// Rename is what makes an interrupted prune safe, so the temporary file must not
// survive the call. Litter next to the history would otherwise accumulate.
func TestPruningLeavesNoTemporaryFileBehind(t *testing.T) {
	home(t)
	path := seed(t, recordAt("ancient", -400*24*time.Hour), recordAt("recent", -time.Hour))

	if err := prune(path, now); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dirOf(t))
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), File+".") {
			t.Errorf("temporary file %q left behind", e.Name())
		}
	}
}

// The rewritten file keeps the mode the original had. CreateTemp makes it 0600,
// and a history that suddenly becomes private would be a surprise beside a
// world-readable config.
func TestPruningKeepsTheFileReadable(t *testing.T) {
	home(t)
	path := seed(t, recordAt("ancient", -400*24*time.Hour), recordAt("recent", -time.Hour))

	if err := prune(path, now); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("mode = %v, want 0644", got)
	}
}

// A record surviving the rewrite must be byte-for-byte what it was, or the
// round trip through the file would be quietly losing something.
func TestPruningPreservesEveryKeptRecord(t *testing.T) {
	home(t)

	want := make([]Record, 0, 2)
	want = append(want,
		Record{Name: "unicode ✓", Focused: 50 * time.Minute, Rounds: 4, EndedAt: now.Add(-3 * time.Hour)},
		Record{Name: "a name with spaces", Focused: 25 * time.Minute, Rounds: 2, EndedAt: now.Add(-2 * time.Hour)},
	)
	path := seed(t, append(want, recordAt("ancient", -400*24*time.Hour))...)

	if err := prune(path, now); err != nil {
		t.Fatal(err)
	}

	got, err := Since(now.AddDate(-100, 0, 0))
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Name != w.Name {
			t.Errorf("record %d name = %q, want %q", i, got[i].Name, w.Name)
		}
		if got[i].Focused != w.Focused {
			t.Errorf("record %d focused = %v, want %v", i, got[i].Focused, w.Focused)
		}
		if got[i].Rounds != w.Rounds {
			t.Errorf("record %d rounds = %d, want %d", i, got[i].Rounds, w.Rounds)
		}
		if !got[i].EndedAt.Equal(w.EndedAt) {
			t.Errorf("record %d ended = %v, want %v", i, got[i].EndedAt, w.EndedAt)
		}
	}
}

// Appending is what triggers pruning in normal use, so the wiring itself is the
// thing being pinned: it is easy to write prune, call it correctly, and still
// never call it.
func TestAppendingPrunesWhatHasAgedOut(t *testing.T) {
	home(t)

	if err := Append(recordAt("ancient", -400*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := Append(recordAt("recent", -time.Hour)); err != nil {
		t.Fatal(err)
	}
	// The record being appended also sets the clock prune measures from.
	if err := Append(recordAt("now", 0)); err != nil {
		t.Fatal(err)
	}

	records, err := Since(now.AddDate(-100, 0, 0))
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range records {
		if r.Name == "ancient" {
			t.Errorf("the old record survived an append")
		}
	}
	if len(records) != 2 {
		t.Errorf("got %d records, want 2", len(records))
	}
}

// A prune that fails must not be reported as a failed append. The record is on
// disk; saying otherwise would be a lie, and a caller that believed it would
// retry and double the record.
//
// The failure is forced with a read-only directory: pruning has to create a
// temporary file beside the history, which that forbids, while appending to a
// file that already exists needs no permission on the directory at all.
func TestAFailedPruneIsNotReportedAsAFailedAppend(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions, so the failure cannot be forced")
	}
	home(t)

	// One ancient record first. Pruning after it is measured from its own
	// EndedAt, so at this point it is the newest thing in the file and stays.
	if err := Append(recordAt("ancient", -400*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	dir := dirOf(t)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	// Appending a record from today now makes the ancient one genuinely due for
	// pruning, so prune has work to do and fails trying to create its temporary
	// file in a directory that cannot be written to.
	if err := Append(recordAt("today", 0)); err != nil {
		t.Fatalf("Append() returned error: %v", err)
	}

	// And the record it reported as written really is on disk.
	records, err := Since(now.AddDate(-100, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Errorf("got %d records, want 2 — the record must survive a failed prune", len(records))
	}
}

func dirOf(t *testing.T) string {
	t.Helper()
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	return path[:len(path)-len(File)]
}
