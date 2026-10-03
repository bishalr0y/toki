package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/bishalr0y/toki/internal/config"
)

// unstyled renders without any colour, so Bar's output is plain text and can be
// measured directly.
var unstyled = lipgloss.NewStyle()

func TestBarFillsToTheGivenFraction(t *testing.T) {
	for _, c := range []struct {
		name      string
		fraction  float64
		wantFill  int
		wantWidth int
	}{
		{"empty at the start", 0, 0, 20},
		{"halfway", 0.5, 10, 20},
		{"full at the end", 1, 20, 20},
		{"past the end is clamped", 1.5, 20, 20},
		{"before the start is clamped", -0.5, 0, 20},
		{"a fraction of a cell rounds down", 0.01, 0, 20},
		{"just over half rounds up", 0.51, 10, 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Bar(c.fraction, 20, unstyled)

			if fill := strings.Count(got, "━"); fill != c.wantFill {
				t.Errorf("filled cells = %d, want %d", fill, c.wantFill)
			}
			if width := lipgloss.Width(got); width != c.wantWidth {
				t.Errorf("width = %d, want %d; the bar must never overflow", width, c.wantWidth)
			}
		})
	}
}

// A zero or negative width would otherwise divide by zero or render nothing.
func TestBarSurvivesAZeroWidth(t *testing.T) {
	if got := lipgloss.Width(Bar(0.5, 0, unstyled)); got != 1 {
		t.Errorf("width = %d, want at least 1", got)
	}
	if got := lipgloss.Width(Bar(0.5, -5, unstyled)); got != 1 {
		t.Errorf("width for a negative width = %d, want at least 1", got)
	}
}

func TestSplitsFromKeepsEveryTimer(t *testing.T) {
	got := SplitsFrom(config.Config{Timers: []config.TimerSplit{
		{Name: "long", FocusMins: 50, BreakMins: 10},
		{Name: "short", FocusMins: 25, BreakMins: 5},
	}})

	if len(got) != 2 {
		t.Fatalf("splits = %d, want 2", len(got))
	}
	if want := (Split{Name: "long", FocusMins: 50, BreakMins: 10}); got[0] != want {
		t.Errorf("first split = %+v, want %+v", got[0], want)
	}
}

// The full description does not fit a narrow terminal, so a row has to drop to a
// compact form rather than run off the edge.
func TestPickerRowsFitTheTerminal(t *testing.T) {
	splits := []Split{{Name: "long", FocusMins: 50, BreakMins: 10}}

	for _, c := range []struct {
		name  string
		width int
		want  []string
	}{
		{"wide shows the durations in full", 80, []string{"long", "50 min focus", "10 min break"}},
		{"narrow stays compact", 30, []string{"long", "50/10 min"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := plain(Picker{Splits: splits, Width: c.width}.View())

			for _, want := range c.want {
				if !strings.Contains(out, want) {
					t.Errorf("picker is missing %q:\n%s", want, out)
				}
			}
			assertFits(t, out, c.width)
		})
	}
}

// A hint that runs off the edge is lost, so it has to wrap instead.
func TestHelpWrapsRatherThanRunningOffTheEdge(t *testing.T) {
	hints := Help(80, [2]string{"space", "pause"}, [2]string{"s", "skip"},
		[2]string{"esc", "back"}, [2]string{"q", "quit"})

	if strings.Contains(hints, "\n") {
		t.Errorf("the hints wrapped at 80 columns, which is plenty:\n%s", hints)
	}

	narrow := Help(24, [2]string{"space", "pause"}, [2]string{"s", "skip"},
		[2]string{"esc", "back"}, [2]string{"q", "quit"})

	if !strings.Contains(narrow, "\n") {
		t.Errorf("the hints did not wrap at 24 columns, so keys would be lost:\n%s", narrow)
	}
	assertFits(t, narrow, 24)

	// Nothing may be dropped, however narrow it gets.
	for _, want := range []string{"space", "pause", "skip", "esc", "back", "quit"} {
		if !strings.Contains(narrow, want) {
			t.Errorf("help lost %q:\n%s", want, narrow)
		}
	}
}

// Wrapping on spaces alone would split the separator and leave a lone "·" at
// the start of a line.
func TestHelpNeverBreaksAPairApartFromItsSeparator(t *testing.T) {
	for _, width := range []int{12, 16, 20, 24, 30, 40} {
		out := Help(width, [2]string{"space", "pause"}, [2]string{"s", "skip"},
			[2]string{"esc", "back"}, [2]string{"q", "quit"})

		for line := range strings.SplitSeq(out, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "·" || strings.HasPrefix(trimmed, "·") {
				t.Errorf("width %d: line %q starts with a stray separator", width, line)
			}
			// A key and its label belong together, so "esc" and "back" must
			// never end up on different lines.
			if strings.Contains(trimmed, "esc") && !strings.Contains(trimmed, "back") {
				t.Errorf("width %d: line %q has a key without its label", width, line)
			}
		}
		assertFits(t, out, width)
	}
}

// The hints have to line up with the rest of the screen, which means leaving
// them unpadded for whatever centres the block. Padding them to the full
// terminal width would leave the centring nothing to do and pin them left.
func TestHelpLeavesCentringToTheCaller(t *testing.T) {
	const width = 40

	wide := Help(width, [2]string{"space", "pause"}, [2]string{"s", "skip"},
		[2]string{"esc", "back"}, [2]string{"q", "quit"})

	for line := range strings.SplitSeq(wide, "\n") {
		if lipgloss.Width(line) >= width {
			t.Errorf("line %q is %d cells, leaving no room to centre it in %d",
				line, lipgloss.Width(line), width)
		}
	}
}

func TestTheSessionScreenNamesItsPhase(t *testing.T) {
	for _, phase := range []string{"FOCUS", "BREAK"} {
		out := plain(Session{Phase: phase, Remaining: "05:00", BarWidth: 20, Width: 80}.View())

		if !strings.Contains(out, phase) {
			t.Errorf("session screen is missing %q:\n%s", phase, out)
		}
		if !strings.Contains(out, "05:00") {
			t.Errorf("session screen is missing the countdown:\n%s", out)
		}
	}
}

// Pausing is reported separately so the phase itself stays readable.
func TestTheSessionScreenShowsBothThePhaseAndThePause(t *testing.T) {
	out := plain(Session{Phase: "FOCUS", Paused: true, Remaining: "12:30", BarWidth: 20, Width: 80}.View())

	for _, want := range []string{"FOCUS", "paused", "12:30"} {
		if !strings.Contains(out, want) {
			t.Errorf("paused session screen is missing %q:\n%s", want, out)
		}
	}
	if got := strings.Count(out, "paused"); got != 1 {
		t.Errorf("paused session screen says paused %d times, want once:\n%s", got, out)
	}
}

func TestTheSummaryReportsNotificationOutcomes(t *testing.T) {
	for _, c := range []struct {
		name   string
		sent   int
		failed int
		want   string
		absent string
	}{
		{"none attempted says nothing at all", 0, 0, "", "notification"},
		{"all sent", 2, 0, "notifications sent", ""},
		{"one failed", 1, 1, "could not be sent", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := plain(Summary{
				SplitName: "long", FocusMins: 50, BreakMins: 10,
				Sent: c.sent, Failed: c.failed,
			}.View())

			if c.want != "" && !strings.Contains(out, c.want) {
				t.Errorf("summary is missing %q:\n%s", c.want, out)
			}
			if c.absent != "" && strings.Contains(out, c.absent) {
				t.Errorf("summary mentions %q, which it should not:\n%s", c.absent, out)
			}
		})
	}
}

func TestAnEmptyConfigIsExplained(t *testing.T) {
	out := plain(Picker{Width: 80}.View())

	if !strings.Contains(out, "no timer splits configured") {
		t.Errorf("an empty picker does not explain itself:\n%s", out)
	}
}

// assertFits checks that no line is wider than the terminal.
func assertFits(t *testing.T, s string, width int) {
	t.Helper()

	for line := range strings.SplitSeq(s, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line %q is %d cells wide, want at most %d", line, w, width)
		}
	}
}

// plain strips styling so assertions can be made about the text itself.
func plain(s string) string {
	var out strings.Builder

	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		out.WriteByte(s[i])
		i++
	}

	return out.String()
}
