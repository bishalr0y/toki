// Command toki is a small Pomodoro timer for the terminal.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bishalr0y/toki/internal/app"
	"github.com/bishalr0y/toki/internal/config"
	historyPkg "github.com/bishalr0y/toki/internal/history"
	"github.com/bishalr0y/toki/internal/ui"
)

// version is overridden at build time with:
//
//	go build -ldflags "-X main.version=1.2.3" ./cmd/toki
//
// It reads "dev" when built straight from source, which is the honest answer: an
// unstamped build is not any particular release.
var version = "dev"

// main reports failures on stderr and exits non-zero, so a broken config is a
// message rather than a full screen interface that half worked.
func main() {
	os.Exit(exit(os.Args[1:], os.Stdout, os.Stderr))
}

// exit runs the program, prints whatever needs printing, and reports the status to
// return.
//
// The exit call is the only part factored out of main, because a test cannot call
// main at all: it reads os.Args and calls os.Exit. Testing run alone was not enough
// to catch a bug that printed "toki: <nil>" on every successful run, since that
// message comes from here rather than from run.
func exit(args []string, stdout, stderr io.Writer) int {
	err := run(args, stdout, stderr)
	if err == nil {
		return 0
	}

	// A bad command line has already been explained, with the usage text, on the
	// way through run. Saying it again here would print every mistake twice, which
	// is exactly what someone reading a wall of text does not need.
	if _, reported := errors.AsType[flagError](err); !reported {
		fmt.Fprintf(stderr, "toki: %v\n", err)
	}
	return 1
}

// flagError marks a command line that could not be parsed. It carries the original
// error so callers can still inspect it, and exists only to say "already reported".
type flagError struct{ err error }

func (e flagError) Error() string { return e.err.Error() }
func (e flagError) Unwrap() error { return e.err }

// run is main's whole body, taking its arguments and its two output streams as
// values so it can be tested without a process or a terminal.
//
// The arguments are handled before anything else happens. Reading the config and
// launching the interface is the last step, not the first, because a program that
// takes a full screen over the terminal has to be sure that is what was wanted:
// `toki --help` landing you in an interactive Pomodoro is disorienting, and it
// buries the typo that caused it.
func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("toki", flag.ContinueOnError)
	// Flag's own diagnostics are suppressed: the error is reported below with the
	// usage text attached, which is more use than "flag provided but not defined"
	// on its own.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	var (
		showHelp    = fs.Bool("help", false, "show this help and exit")
		showVersion = fs.Bool("version", false, "print the version and exit")
		listSplits  = fs.Bool("list", false, "print the configured splits and exit")
		showToday   = fs.Bool("stats", false, "show today's focus and round count")
		showHistory = fs.Bool("history", false, "show every recorded split in the last 3 months")
	)
	fs.BoolVar(showHelp, "h", false, "show this help and exit")

	if err := fs.Parse(args); err != nil {
		// The mistake first, then the way out. Printed here rather than left to
		// main, so the two appear together and in that order.
		fmt.Fprintf(stderr, "toki: %v\n\n", err)
		usage(stderr)
		return flagError{err}
	}

	switch {
	case *showHelp:
		usage(stdout)
		return nil
	case *showVersion:
		fmt.Fprintf(stdout, "toki %s\n", version)
		return nil
	case *listSplits:
		return list(stdout)
	}

	// The statistics are printed rather than drawn, like --list.
	//
	// A full screen interface to answer "how much did I do today" would be absurd:
	// it would have to be drawn, redrawn, and then dismissed before the number
	// could be read, and the answer would be impossible to pipe or grep. These are
	// questions asked from a shell, so they are answered from a shell.
	if *showToday || *showHistory {
		return report(stdout, *showToday, *showHistory)
	}

	// Read once at startup. The old menu re-read the file on every iteration,
	// so edits made mid-session silently changed what was on screen.
	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}

	return app.Run(cfg)
}

// report prints the requested statistics and exits.
//
// Both can be asked for at once, and then both are printed: each answers a
// different question, and neither subsumes the other. Today comes first because
// it is the figure most likely to be wanted.
func report(out io.Writer, today, history bool) error {
	now := time.Now()

	var sections []string

	if today {
		totals, err := historyPkg.Today(now)
		if err != nil {
			return err
		}
		sections = append(sections, ui.ReportSection("today", ui.TodayTable(totals)))
	}

	if history {
		records, err := historyPkg.Recent(now)
		if err != nil {
			return err
		}

		days := historyPkg.GroupByDay(records)

		var total historyPkg.Totals
		for _, d := range days {
			total.Focused += d.Total.Focused
			total.Rounds += d.Total.Rounds
			total.Splits += d.Total.Splits
		}

		heading := fmt.Sprintf("last %d months", historyPkg.RetentionMonths)
		sections = append(sections, ui.ReportSection(heading, ui.HistoryTable(days, total)))
	}

	// One blank line before every heading, the first included.
	//
	// The leading newline is what gives the first heading the same breathing room
	// as the rest: without it the report opens on a heading in the very first
	// column, which reads as though the top of it had been cut off. Two newlines
	// between the sections then make every heading sit after exactly one blank
	// line rather than the first being tighter than the ones that follow.
	fmt.Fprint(out, "\n"+strings.Join(sections, "\n\n")+"\n")

	return nil
}

// list prints the configured splits and exits.
//
// This answers "did my config edit work?" without taking over the screen, which is
// the only way to find out from a shell. The durations are printed as they will
// actually run rather than as they are written, because a config that leaves the
// round count unset runs four rounds and should say so.
func list(out io.Writer) error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}

	path, err := config.Dir()
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Splits from %s:\n\n", filepath.Join(path, "config.yaml"))
	for i, split := range cfg.Timers {
		fmt.Fprintf(out, "  %d  %-20s %d × %d min  ·  %d min break, %d min long\n",
			i+1, split.Name, split.Cycles, split.FocusMins, split.BreakMins, split.LongBreakMins)
	}
	return nil
}

// usage prints the help text.
//
// It carries the keys as well as the flags, because this is the only description of
// them that exists anywhere outside the running interface, and the interface is not
// available to someone who has just mistyped a flag.
func usage(out io.Writer) {
	fmt.Fprint(out, `toki — a Pomodoro timer for the terminal.

Usage:
  toki              start the interface and pick a split
  toki --stats      print today's focus and round count
  toki --history    print every recorded split in the last 3 months
  toki --list       print the configured splits and exit
  toki --help       show this help and exit
  toki --version    print the version and exit

Keys:
  ↑ ↓  move          space  pause or resume    s  skip the phase
  1-9  start that    enter  start              esc  abandon the split
  q    quit from anywhere

Splits are read from ~/.config/toki/config.yaml, which is created with sensible
defaults the first time toki runs. See the README for the available keys.
`)
}
