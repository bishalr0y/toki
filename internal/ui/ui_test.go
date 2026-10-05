package ui

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/bishalr0y/toki/internal/timer"

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
	splits := []Split{
		{Name: "long", FocusMins: 50, BreakMins: 10, Cycles: 4, LongBreakMins: 20},
		{Name: "short", FocusMins: 25, BreakMins: 5},
	}

	for _, c := range []struct {
		name  string
		width int
		want  []string
	}{
		{
			"wide shows the rhythm in full", 80,
			[]string{"long", "4 × 50 min", "10 min break, 20 min long"},
		},
		{"narrow stays compact", 30, []string{"long", "4×50 min"}},
		// A split that says nothing about cycles still runs one round rather than
		// none, so it must not read as zero.
		{"unset cycles reads as one round", 30, []string{"short", "1×25 min"}},
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
	for _, phase := range []timer.Phase{timer.PhaseFocus, timer.PhaseBreak} {
		out := plain(Session{Phase: phase, Remaining: "05:00", BarWidth: 20, Width: 80}.View())

		if !strings.Contains(out, phase.String()) {
			t.Errorf("session screen is missing %q:\n%s", phase, out)
		}
		if !strings.Contains(out, "05:00") {
			t.Errorf("session screen is missing the countdown:\n%s", out)
		}
	}
}

// Pausing is reported separately so the phase itself stays readable.
func TestTheSessionScreenShowsBothThePhaseAndThePause(t *testing.T) {
	out := plain(Session{Phase: timer.PhaseFocus, Paused: true, Remaining: "12:30", BarWidth: 20, Width: 80}.View())

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
				SplitName: "long", Rounds: 4, Focus: "3h 20m",
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

// A warning is worth saying out loud: a user who believes a notification was sent
// when it was not has no other way to find out, and a split that ended silently is
// a split that is easy to miss.
func TestTheSummaryReportsAFailedNotification(t *testing.T) {
	// The width is set because a screen that was not told how wide it is has no
	// width to wrap a warning to, and falls back to one cell. Left unset, this would
	// have reported a missing notification for the wrong reason.
	out := plain(Summary{
		SplitName: "long", Rounds: 1, Focus: "25m", Width: 80,
		Warning: `desktop notification failed: exec: "notify-send": not found`,
	}.View())

	if !strings.Contains(out, "notify-send") {
		t.Errorf("summary hides the failure to notify:\n%s", out)
	}
}

// The round count is only shown while working: over a break there is no round in
// progress to count.
func TestTheSessionScreenShowsTheRoundWhileWorking(t *testing.T) {
	withRound := plain(Session{Phase: timer.PhaseFocus, Round: "round 2 of 4", BarWidth: 20}.View())
	if !strings.Contains(withRound, "round 2 of 4") {
		t.Errorf("the session screen hides the round:\n%s", withRound)
	}

	// A break, even if a round label were somehow supplied.
	onBreak := plain(Session{Phase: timer.PhaseBreak, Round: "round 2 of 4", BarWidth: 20}.View())
	if strings.Contains(onBreak, "round") {
		t.Errorf("the session screen shows a round during a break:\n%s", onBreak)
	}
}

// A long break is a rest phase like any other, so the bar is styled as one. It
// used to fall through to the focus bar because the comparison only knew about
// "BREAK".
func TestTheLongBreakIsDrawnAsRestRatherThanWork(t *testing.T) {
	onLongBreak := plain(Session{Phase: timer.PhaseLongBreak, BarWidth: 20}.View())
	onFocus := plain(Session{Phase: timer.PhaseFocus, BarWidth: 20}.View())

	if onLongBreak == onFocus {
		t.Error("the long break is drawn identically to focus")
	}
	if !strings.Contains(onLongBreak, "LONG BREAK") {
		t.Errorf("the long break is not named on screen:\n%s", onLongBreak)
	}
}

// A name longer than the column padding pushes the row past the terminal, where
// the tail is simply lost. Choosing the wide layout by a fixed guess cannot catch
// that, so the label has to be measured.
//
// Only the rows are checked: the banner is fixed decoration at 72 cells and
// cannot narrow to suit.
func TestAPickerRowNeverRunsOffTheEdgeWhateverTheName(t *testing.T) {
	long := Split{
		Name:          "an extremely long split name that will not fit anywhere",
		FocusMins:     50,
		BreakMins:     10,
		Cycles:        4,
		LongBreakMins: 20,
	}

	for _, width := range []int{20, 30, 40, 46, 60, 80, 120} {
		out := plain(Picker{Splits: []Split{long}, Width: width}.View())

		for line := range strings.SplitSeq(out, "\n") {
			if !strings.Contains(line, "fit anywhere") {
				continue
			}
			if got := lipgloss.Width(line); got > width {
				t.Errorf("at %d columns the row is %d cells and runs off the edge:\n%s",
					width, got, line)
			}
		}
	}
}

// The rhythm is the thing to compare splits on, so it has to be on every row,
// including the compact ones.
func TestThePickerShowsTheRhythmAtEveryWidth(t *testing.T) {
	splits := []Split{{Name: "long", FocusMins: 50, BreakMins: 10, Cycles: 4, LongBreakMins: 20}}

	for _, width := range []int{20, 30, 46, 60, 80} {
		out := plain(Picker{Splits: splits, Width: width}.View())
		if !strings.Contains(out, "4") || !strings.Contains(out, "50") {
			t.Errorf("width %d loses the rhythm:\n%s", width, out)
		}
	}
}

// "1 splits" is the kind of detail that makes a summary look unfinished.
func TestTheSummaryCountsInTheRightNumber(t *testing.T) {
	if one := plain(Summary{Rounds: 1, Focus: "25m"}.View()); strings.Contains(one, "1 rounds") {
		t.Errorf("summary says \"1 rounds\":\n%s", one)
	}

	many := plain(Summary{Rounds: 3, Focus: "1h 15m"}.View())
	if !strings.Contains(many, "3 rounds") {
		t.Errorf("summary is missing \"3 rounds\":\n%s", many)
	}
}

// Nothing may ask for a fixed RGB value.
//
// A hex colour is a guess about what the terminal's background is, and it is wrong
// the moment that background is not the one it was designed against: unreadable text
// on a light theme, a bar that vanishes on a dark one. Restricting the interface to
// the sixteen colours every terminal provides is what makes it look deliberate on all
// of them, because in every case the terminal decides the actual shade.
//
// This is the check that keeps that true. It walks the rendered screens rather than
// the styles, so a colour hardcoded anywhere else would be caught too.
func TestNoScreenAsksForAFixedColourValue(t *testing.T) {
	splits := []Split{{Name: "long", FocusMins: 50, BreakMins: 10, Cycles: 4, LongBreakMins: 20}}

	for _, screen := range []struct {
		what string
		out  string
	}{
		{"picker", Picker{Splits: splits, Cursor: 0, Width: 80}.View()},
		{"focus", Session{
			SplitName: "long", Phase: timer.PhaseFocus, Round: "round 1 of 4",
			Remaining: "25:00", ElapsedFrac: 0.4, BarWidth: 20, Width: 80,
		}.View()},
		{"break", Session{
			SplitName: "long", Phase: timer.PhaseBreak, Paused: true,
			Remaining: "05:00", BarWidth: 20, Width: 80, Warning: "careful",
		}.View()},
		{"summary", Summary{
			SplitName: "long", Rounds: 4, Focus: "3h 20m", Sent: 2, Width: 80,
		}.View()},
		{"help", Help(80, [2]string{"space", "pause"}, [2]string{"t", "theme"},
			[2]string{"q", "quit"})},
	} {
		for _, code := range sgrParams(screen.out) {
			if strings.HasPrefix(code, "38;2;") || strings.HasPrefix(code, "48;2;") {
				t.Errorf("the %s screen asks for a fixed colour %q; only the sixteen are portable",
					screen.what, code)
			}
		}
	}
}

// Body text inherits the terminal's own foreground, which is the one colour
// guaranteed to contrast with its own background whatever the theme. Naming a colour
// would throw that guarantee away for no gain.
func TestBodyTextInheritsTheTerminalsOwnForeground(t *testing.T) {
	for _, c := range []struct {
		what string
		out  string
	}{
		{"body text", TextStyle.Render("X")},
		{"picker row", RowStyle.Render("X")},
		{"key hint", KeyStyle.Render("X")},
	} {
		for _, code := range sgrParams(c.out) {
			// 30-37 are the dark foregrounds and 90-97 the bright ones; anything in
			// either range would be a specific colour rather than the terminal's own.
			if isColourParam(code) {
				t.Errorf("%s sets colour %q, want the terminal's default foreground", c.what, code)
			}
		}
	}
}

// The highlighted row has to stay legible on whatever the terminal calls bright
// yellow. Themes put light colours in the bright half of the palette, so a dark
// foreground on a bright background is the one pairing that holds everywhere.
func TestTheHighlightedRowIsDarkTextOnABrightBar(t *testing.T) {
	codes := sgrParams(SelectedStyle.Render("X"))

	var dark, bright bool
	for _, param := range codes {
		for n := range strings.SplitSeq(param, ";") {
			switch {
			case n >= "30" && n <= "37":
				dark = true
			case n >= "100" && n <= "107":
				bright = true
			}
		}
	}

	if !dark {
		t.Errorf("the selected row has no dark foreground, only %v", codes)
	}
	if !bright {
		t.Errorf("the selected row has no bright background, only %v", codes)
	}
}

// Two roles sharing a colour would make the interface read as one undifferentiated
// block, so each accent has to be distinct from the others.
func TestEveryAccentIsADistinctColour(t *testing.T) {
	accents := map[string]string{
		"blue":  lipgloss.NewStyle().Foreground(Blue).Render("X"),
		"red":   lipgloss.NewStyle().Foreground(Red).Render("X"),
		"green": lipgloss.NewStyle().Foreground(Green).Render("X"),
		"mauve": lipgloss.NewStyle().Foreground(Mauve).Render("X"),
		"peach": lipgloss.NewStyle().Foreground(Peach).Render("X"),
	}

	seen := map[string]string{}
	for name, out := range accents {
		if other, dup := seen[out]; dup {
			t.Errorf("%s and %s render the same colour %q", name, other, out)
		}
		seen[out] = name
	}
}

// The phase is meant to be readable without reading the label, which stops being true
// if focus and break end up the same colour.
func TestFocusAndBreakAreDrawnInDifferentColours(t *testing.T) {
	if FocusLabel.Render("X") == BreakLabel.Render("X") {
		t.Error("the focus and break labels are drawn in the same colour")
	}
	if FocusBar.Render("━") == BreakBar.Render("━") {
		t.Error("the focus and break bars are drawn in the same colour")
	}
}

// Quieter lines have to look quieter. Body text uses the terminal's foreground and
// the subdued lines ask for its dim colour, so the two are distinguishable in either
// direction.
func TestSubduedLinesLookQuieterThanBodyText(t *testing.T) {
	if len(sgrParams(TextStyle.Render("X"))) != len(sgrParams(RowStyle.Render("X"))) {
		t.Error("body text and picker rows are styled differently")
	}

	subtle := sgrParams(Subtle.Render("X"))
	if len(subtle) == 0 {
		t.Error("subdued lines are painted in the same colour as body text")
	}
}

// sgrParams returns the numeric parameters of every escape sequence in a rendered
// string, so a test can talk about colours rather than about bytes.
func sgrParams(s string) []string {
	var out []string

	for i := 0; i < len(s); {
		if s[i] != 0x1b {
			i++
			continue
		}
		j := i + 1
		for j < len(s) && (s[j] == '[' || s[j] == ';') {
			j++
		}
		start := j
		for j < len(s) && s[j] != 'm' {
			j++
		}
		if j < len(s) {
			if params := s[start:j]; params != "" {
				out = append(out, params)
			}
		}
		i = j + 1
	}

	return out
}

// isColourParam reports whether an SGR parameter sets a colour, rather than an
// attribute like bold or reverse video.
func isColourParam(param string) bool {
	if i := strings.IndexByte(param, ';'); i >= 0 {
		param = param[:i]
	}
	n, err := strconv.Atoi(param)
	if err != nil {
		return false
	}
	return (n >= 30 && n <= 37) || (n >= 40 && n <= 47) ||
		(n >= 90 && n <= 97) || (n >= 100 && n <= 107)
}

// A long break is a rest, and has to be drawn as one.
//
// It used to be labelled in the focus colour while its bar was drawn in the break
// colour, because one place asked whether the phase was "BREAK" and the other asked
// whether it was "BREAK" or "LONG BREAK". The two answers drifted apart the moment a
// second kind of break existed.
func TestALongBreakIsDrawnLikeTheShortOne(t *testing.T) {
	long := sgrParams(LabelFor(timer.PhaseLongBreak))
	short := sgrParams(LabelFor(timer.PhaseBreak))
	work := sgrParams(LabelFor(timer.PhaseFocus))

	if !slices.Equal(long, short) {
		t.Errorf("the long break is drawn %v but the short one %v", long, short)
	}
	if slices.Equal(long, work) {
		t.Errorf("the long break is drawn in the focus colour %v", long)
	}
}

// Every phase the timer can produce must be drawn in the colour of its kind, and
// the label and the bar must agree about which kind that is.
//
// The original bug was precisely a disagreement: the label asked whether the phase
// was "BREAK" and the bar asked whether it was "BREAK" or "LONG BREAK", so a long
// break came out labelled as work and drawn as rest. Checking both against the same
// written-down rule catches that shape of mistake, not just this instance of it.
//
// The list is taken from the timer rather than spelled out as strings, so the
// classification cannot drift away from the phases that actually exist.
func TestEveryPhaseIsDrawnInTheColourOfItsKind(t *testing.T) {
	for _, c := range []struct {
		phase timer.Phase
		rest  bool // stated here rather than asked of the code under test
	}{
		{timer.PhaseIdle, false},
		{timer.PhaseFocus, false},
		{timer.PhaseBreak, true},
		{timer.PhaseLongBreak, true},
	} {
		name := c.phase.String()

		wantBar, otherBar := FocusBar, BreakBar
		wantLabel := FocusLabel
		if c.rest {
			wantBar, otherBar = BreakBar, FocusBar
			wantLabel = BreakLabel
		}

		if got := LabelFor(c.phase); got != wantLabel.Render(name) {
			t.Errorf("phase %v is labelled %q, want the style of a %s",
				c.phase, got, kind(c.rest))
		}

		view := Session{Phase: c.phase, BarWidth: 10, Width: 80}.View()
		if !strings.Contains(view, Bar(0, 10, wantBar)) {
			t.Errorf("phase %v does not draw its bar in the %s colour:\n%s",
				c.phase, kind(c.rest), plain(view))
		}
		if strings.Contains(view, Bar(0, 10, otherBar)) {
			t.Errorf("phase %v draws its bar in the %s colour as well:\n%s",
				c.phase, kind(!c.rest), plain(view))
		}
	}
}

// kind names a phase kind for an error message.
func kind(rest bool) string {
	if rest {
		return "rest"
	}
	return "work"
}

// Nothing may render wider than the terminal it was given, at any width.
//
// This is a sweep rather than a handful of fixed widths because the overflows did
// not sit where a spot check would have found them. The picker measured the rhythm
// text but not the two cells of padding either side of every row, so it overflowed
// in a band roughly four columns wide — a band that a test at 30 and a test at 60
// would both have missed. Only walking every width finds a band.
//
// A line that is too wide does not merely look wrong: it wraps, and the wrapped
// remainder is drawn where the next line goes, which pushes everything below it out
// of alignment for the rest of the session.
//
// The floor is the width of the longest fixed string any screen has to show, which
// on the summary is "4 rounds  ·  1h 40m focused" at 27 cells. Below that something
// must wrap, and wrapping is worse than useless here: the remainder is drawn over
// the line below, so the screen stops lining up. A terminal narrower than its own
// summary text is not a case worth engineering for, and the limit is stated here
// rather than left to be discovered.
func TestNoScreenOverflowsAtAnyWidth(t *testing.T) {
	const minWidth = 27

	splits := []Split{
		{Name: "Classic", FocusMins: 25, BreakMins: 5, Cycles: 4, LongBreakMins: 15},
		{Name: "A Really Extremely Long Split Name", FocusMins: 50, BreakMins: 10, Cycles: 3, LongBreakMins: 20},
	}

	for width := minWidth; width <= 120; width++ {
		barWidth := min(44, max(width-10, 4))

		screens := []struct {
			what string
			out  string
		}{
			{"picker, short name", Picker{
				Splits: splits[:1], Cursor: 0, Width: width,
			}.View()},
			{"picker, long name", Picker{
				Splits: splits, Cursor: 1, Width: width,
			}.View()},
			{"session", Session{
				SplitName: splits[1].Name, Phase: timer.PhaseFocus, Round: "round 1 of 3",
				Remaining: "25:00", ElapsedFrac: 0.4, BarWidth: barWidth, Width: width,
			}.View()},
			{"session, paused with a warning", Session{
				SplitName: splits[1].Name, Phase: timer.PhaseBreak, Paused: true,
				Remaining: "05:00", BarWidth: barWidth, Width: width,
				Warning: "desktop notification failed: notify-send: not found",
			}.View()},
			{"summary", Summary{
				SplitName: splits[1].Name, Rounds: 4, Focus: "1h 40m",
				Sent: 2, Width: width,
			}.View()},
			{"help", Help(width,
				[2]string{"space", "pause"}, [2]string{"s", "skip"},
				[2]string{"esc", "back"}, [2]string{"q", "quit"})},
		}

		for _, screen := range screens {
			for line := range strings.SplitSeq(screen.out, "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Errorf("at %d columns the %s is %d cells wide:\n  %q",
						width, screen.what, got, line)
				}
			}
		}
	}
}

// A warning long enough to run off the edge must not take the screen with it.
//
// The message is program text, not something the user wrote, so it can be arbitrarily
// long on its own: a long path, a verbose error from the filesystem.
//
// Wrapping is the right treatment here precisely because a filesystem error ends in
// its cause — "permission denied", the path it applies to — and clipping would drop
// exactly the part worth reading. So the assertion is that not one character is lost,
// and it ignores whitespace to say that.
//
// Ignoring whitespace is not a convenience. A path longer than the terminal cannot
// help but be split across lines, and so can the phrase "permission denied" when it
// lands on a line boundary. Both are still fully readable; a message missing its
// cause is not. Checking for a substring would fail on a rendering that is perfectly
// readable, so it would be testing the wrong thing.
func TestAnOverlongWarningLosesNothing(t *testing.T) {
	const width = 40

	message := "desktop notification failed: " +
		"exec /home/someone/with/a/very/long/path/to/bin/notify-send: " +
		"permission denied"

	out := Session{
		SplitName: "Classic", Phase: timer.PhaseFocus, Remaining: "25:00",
		BarWidth: 20, Width: width, Warning: message,
	}.View()

	assertFits(t, out, width)

	// The warning is the only part of this screen carrying the message.
	rendered := squeeze(plain(out[strings.Index(plain(out), "desktop"):]))
	if want := squeeze("⚠  " + message); rendered != want {
		t.Errorf("the warning did not survive intact\n got: %s\nwant: %s", rendered, want)
	}
}

// A screen told nothing about the terminal's width must still render a readable
// warning, not one character per line.
//
// This is what the zero value of Width used to do: wrapping to zero cells is
// technically correct and completely useless, and it read as a crash rather than as
// a bug. The real program always passes a width, so this only covers a screen built
// without one — but it is worth pinning, because the failure is silent and ugly.
func TestAWarningIsReadableWhenNoWidthWasGiven(t *testing.T) {
	out := plain(Summary{
		SplitName: "Classic", Rounds: 1, Focus: "25m",
		Warning: "desktop notification failed: exec: \"notify-send\": not found",
	}.View())

	if !strings.Contains(out, "notify-send") {
		t.Errorf("without a width the warning is unreadable:\n%s", out)
	}
	if strings.Contains(out, "n\no\nt\ni") {
		t.Errorf("without a width the warning is broken one character per line:\n%s", out)
	}
}

// squeeze removes every space from s, so a comparison can tell whether text was lost
// without caring where the lines happened to break.
func squeeze(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' {
			return -1
		}
		return r
	}, s)
}

// The warning is the longest line on any screen, so it is what runs off the edge
// first. Sweeping widths is what catches it, because the picker overflowed in a band
// of four columns that a test at any one width would have stepped straight over.
func TestAWarningFitsAtEveryWidth(t *testing.T) {
	message := "desktop notification failed: " +
		"exec /home/someone/with/a/very/long/path/to/bin/notify-send: " +
		"permission denied"

	for width := 27; width <= 120; width++ {
		out := Session{
			SplitName: "Classic", Phase: timer.PhaseFocus, Remaining: "25:00",
			BarWidth: min(44, max(width-10, 4)), Width: width, Warning: message,
		}.View()

		assertFits(t, out, width)
	}
}

// The complaint has to say how much room is needed, or the user has no way to know how
// far to resize. This is better than rendering a cramped screen: a picker whose rows
// are cut off mid-way through a list looks like a bug, and a help line the user cannot
// see is a feature they cannot use.
func TestTooSmallSaysHowMuchRoomIsNeeded(t *testing.T) {
	out := squeeze(plain(TooSmall(20)))

	for _, want := range []string{"windowtoosmall", "need27wide", "have20"} {
		if !strings.Contains(out, want) {
			t.Errorf("the message is missing %q:\n%s", want, out)
		}
	}
}

// A window that is too short rather than too narrow must not be told to get wider.
//
// The width is not what is wrong in that case, and quoting it anyway would send the
// user off to widen a window that is already wide enough — while saying nothing about
// the height, which is the dimension they actually have to change.
func TestATooShortWindowIsNotToldToGetWider(t *testing.T) {
	out := squeeze(plain(TooSmallTall(80, 9, 14)))

	for _, want := range []string{"need14tall", "have9"} {
		if !strings.Contains(out, want) {
			t.Errorf("the message is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "wide") {
		t.Errorf("a window that is wide enough is being told about width:\n%s", out)
	}
}

// The height quoted has to be the one that was actually needed. It is measured from
// the rendered screen rather than declared as a constant, so the message stays true as
// the picker grows by a row per split and warnings add rows of their own.
func TestTheHeightQuotedIsTheHeightActuallyNeeded(t *testing.T) {
	out := plain(TooSmallTall(80, 9, 14))

	if !strings.Contains(out, "14") {
		t.Errorf("the message does not quote the height it was given:\n%s", out)
	}
}

// The message has to fit inside the window that triggered it, which is the one
// place that is guaranteed to be too small for anything.
func TestTooSmallFitsInsideTheWindowThatAskedForIt(t *testing.T) {
	for _, size := range [][2]int{{20, 5}, {1, 1}, {10, 3}, {40, 2}} {
		assertFits(t, TooSmall(size[0]), size[0])
		assertFits(t, TooSmallTall(size[0], size[1], 30), size[0])
	}
}

// The message must not be centred into a window it cannot fit in, which is what
// lipgloss's centring would otherwise do by padding to the widest line.
func TestTooSmallNeverExceedsItsOwnWidth(t *testing.T) {
	for width := 1; width <= 60; width++ {
		for _, out := range []string{TooSmall(width), TooSmallTall(width, 3, 99)} {
			if got := lipgloss.Width(out); got > width {
				t.Errorf("a complaint in %d columns is %d cells wide:\n%q", width, got, out)
			}
		}
	}
}

// A count of one has to read in the singular, on whichever screen reports a count.
func TestACountOfOneIsNotPluralised(t *testing.T) {
	for _, c := range []struct {
		n    int
		want string
	}{{1, "1 round"}, {2, "2 rounds"}, {0, "0 rounds"}} {
		out := plain(Summary{SplitName: "x", Rounds: c.n, Focus: "25m", Width: 80}.View())
		if !strings.Contains(out, c.want) {
			t.Errorf("with Rounds=%d the summary should say %q:\n%s", c.n, c.want, out)
		}
	}
}
