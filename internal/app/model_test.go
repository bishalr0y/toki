package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bishalr0y/toki/internal/config"
	"github.com/bishalr0y/toki/internal/timer"
)

// start is the fixed instant every test anchors to, so expectations can be
// absolute clock readings rather than values recomputed the way the code
// recomputes them.
var start = time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)

func testConfig() config.Config {
	return config.Config{Timers: []config.TimerSplit{
		{Name: "Classic", FocusMins: 25, BreakMins: 5, Cycles: 2, LongBreakMins: 15},
		{Name: "Long", FocusMins: 50, BreakMins: 10, Cycles: 4, LongBreakMins: 20},
	}}
}

// newTestModel returns a model whose clock is frozen at start and whose sound is
// a no-op, so a session ending does not reach for the developer's speakers.
func newTestModel(t *testing.T) Model {
	t.Helper()

	m := New(testConfig())
	m.now = func() time.Time { return start }
	m.play = func() {}

	return m
}

// at returns the same model with its clock moved to an instant, so the next
// keypress is handled as though that were the current time.
func at(m Model, now time.Time) Model {
	m.now = func() time.Time { return now }
	return m
}

// key builds the message the terminal would deliver for a named key.
func key(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
	}
}

// send pushes one message through Update and returns the resulting model,
// discarding the command.
func send(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()

	model, _ := sendCmd(t, m, msg)
	return model
}

func sendCmd(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()

	next, cmd := m.Update(msg)
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want app.Model", next)
	}
	return got, cmd
}

func press(t *testing.T, m Model, name string) Model {
	t.Helper()
	return send(t, m, key(name))
}

// tickAt pushes a redraw message for a given instant and returns the commands
// the update asked for.
func tickAt(t *testing.T, m Model, now time.Time) (Model, tea.Cmd) {
	t.Helper()
	return sendCmd(t, m, tickMsg(now))
}

func TestThePickerOpensOnTheFirstSplit(t *testing.T) {
	m := newTestModel(t)

	if m.screen != screenPicker {
		t.Errorf("screen = %v, want the picker", m.screen)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if len(m.splits) != 2 {
		t.Fatalf("splits = %d, want 2", len(m.splits))
	}
	if m.splits[0].Name != "Classic" {
		t.Errorf("first split = %q, want Classic", m.splits[0].Name)
	}
}

func TestTheCursorStopsAtTheEnds(t *testing.T) {
	m := newTestModel(t)

	if got := press(t, m, "up"); got.cursor != 0 {
		t.Errorf("cursor after up at the top = %d, want 0", got.cursor)
	}

	m = press(t, m, "down")
	if m.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", m.cursor)
	}
	if got := press(t, m, "down"); got.cursor != 1 {
		t.Errorf("cursor after down at the bottom = %d, want 1", got.cursor)
	}

	m = press(t, m, "k")
	if m.cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", m.cursor)
	}
	if got := press(t, m, "j"); got.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", got.cursor)
	}
}

func TestEnterStartsTheHighlightedSplit(t *testing.T) {
	m := press(t, press(t, newTestModel(t), "down"), "enter")

	if m.screen != screenSession {
		t.Fatalf("screen = %v, want the session", m.screen)
	}
	if m.name != "Long" {
		t.Errorf("name = %q, want Long", m.name)
	}
	if m.timer.Phase() != timer.PhaseFocus {
		t.Errorf("phase = %v, want focus", m.timer.Phase())
	}
	if got := m.timer.Remaining(start); got != 50*time.Minute {
		t.Errorf("Remaining = %v, want 50m", got)
	}
}

func TestANumberKeyStartsThatSplitDirectly(t *testing.T) {
	m := press(t, newTestModel(t), "2")

	if m.screen != screenSession {
		t.Fatalf("screen = %v, want the session", m.screen)
	}
	if m.name != "Long" {
		t.Errorf("name = %q, want Long", m.name)
	}
}

func TestANumberKeyWithNoSuchRowIsIgnored(t *testing.T) {
	m := press(t, newTestModel(t), "5")

	if m.screen != screenPicker {
		t.Errorf("screen = %v, want the picker; only two rows exist", m.screen)
	}
}

func TestQAndCtrlCQuit(t *testing.T) {
	for _, name := range []string{"q", "ctrl+c"} {
		t.Run(name, func(t *testing.T) {
			_, cmd := newTestModel(t).Update(key(name))
			if cmd == nil {
				t.Fatalf("%s returned no command, want a quit", name)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("%s produced %T, want tea.QuitMsg", name, cmd())
			}
		})
	}
}

func TestSpacePausesAndSpaceResumes(t *testing.T) {
	// Twelve and a half minutes into the session.
	m := at(press(t, newTestModel(t), "enter"), start.Add(12*time.Minute+30*time.Second))

	m = press(t, m, "space")
	if !m.timer.Paused() {
		t.Fatal("Paused() = false after space, want true")
	}
	if got, want := m.timer.Remaining(start.Add(time.Hour)), 12*time.Minute+30*time.Second; got != want {
		t.Errorf("Remaining while paused = %v, want a frozen %v", got, want)
	}

	m = press(t, at(m, start.Add(13*time.Minute)), "space")
	if m.timer.Paused() {
		t.Error("Paused() = true after a second space, want false")
	}

	// The frozen 12m30s comes back rather than the 25m the phase began with.
	resumedAt := start.Add(13 * time.Minute)
	if got, want := m.timer.Remaining(resumedAt), 12*time.Minute+30*time.Second; got != want {
		t.Errorf("Remaining after resuming = %v, want the frozen %v", got, want)
	}
	// And it is running again, not frozen.
	if got, want := m.timer.Remaining(resumedAt.Add(time.Minute)), 11*time.Minute+30*time.Second; got != want {
		t.Errorf("Remaining a minute later = %v, want %v; the clock is running again", got, want)
	}
}

// A manual skip deliberately starts the next phase from now; Advance is for
// phases that merely ran out.
func TestSkipMovesStraightToTheBreakWithAFullDuration(t *testing.T) {
	m := press(t, at(press(t, newTestModel(t), "enter"), start.Add(10*time.Minute)), "s")

	if m.screen != screenSession {
		t.Fatalf("screen = %v, want the session to carry on", m.screen)
	}
	if m.timer.Phase() != timer.PhaseBreak {
		t.Errorf("phase = %v, want break", m.timer.Phase())
	}
	if got := m.timer.Remaining(start.Add(10 * time.Minute)); got != 5*time.Minute {
		t.Errorf("Remaining = %v, want a full 5m from now", got)
	}
}

// A split runs several phases now, so the summary only appears once the last of
// them is skipped, not after the first break.
func TestSkippingTheLastPhaseShowsTheSummary(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	// Two rounds of focus with a break between, then the long break.
	for i := range 4 {
		m = skip(t, m, start.Add(time.Duration(i)*40*time.Minute))
	}

	if m.screen != screenSummary {
		t.Errorf("screen = %v, want the summary", m.screen)
	}
	if !m.timer.Finished() {
		t.Error("Finished() = false, want true")
	}
}

// Reaching the summary early is not a thing: skipping the first break returns to
// focus rather than ending a two round split.
func TestSkippingTheFirstBreakCarriesOnIntoTheNextRound(t *testing.T) {
	m := skip(t, press(t, newTestModel(t), "enter"), start) // focus into break
	m = skip(t, m, start.Add(25*time.Minute))               // break into focus

	if m.screen != screenSession {
		t.Fatalf("screen = %v, want the session to carry on", m.screen)
	}
	if m.timer.Phase() != timer.PhaseFocus {
		t.Errorf("phase = %v, want focus", m.timer.Phase())
	}
}

func TestEscapeGoesBackToThePicker(t *testing.T) {
	m := press(t, press(t, newTestModel(t), "enter"), "esc")

	if m.screen != screenPicker {
		t.Errorf("screen = %v, want the picker", m.screen)
	}
}

// Leaving and starting again must begin a clean session, with the full focus
// duration.
func TestStartingAgainAfterLeavingCarriesNoStaleState(t *testing.T) {
	m := press(t, newTestModel(t), "enter")
	m = press(t, m, "s") // into the break
	m = press(t, m, "esc")

	m = press(t, at(m, start.Add(3*time.Hour)), "enter")

	if m.screen != screenSession {
		t.Fatalf("screen = %v, want the session", m.screen)
	}
	if m.timer.Phase() != timer.PhaseFocus {
		t.Errorf("phase = %v, want focus", m.timer.Phase())
	}
	if got, want := m.timer.Remaining(start.Add(3*time.Hour)), 25*time.Minute; got != want {
		t.Errorf("Remaining = %v, want the full %v", got, want)
	}
}

func TestTheSummaryGoesBackToThePicker(t *testing.T) {
	m := walkToSummary(t, newTestModel(t), start)

	m = press(t, m, "enter")

	if m.screen != screenPicker {
		t.Errorf("screen = %v, want the picker", m.screen)
	}
}

func TestAPhaseThatRunsOutPlaysASoundAndAdvances(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	// One second past the focus deadline.
	m, cmd := tickAt(t, m, start.Add(25*time.Minute+time.Second))

	if m.timer.Phase() != timer.PhaseBreak {
		t.Errorf("phase = %v, want break", m.timer.Phase())
	}
	if got := soundsPlayed(cmd); got != 1 {
		t.Errorf("played %d sounds, want 1 for the phase that ran out", got)
	}
}

// A model built the way the program builds it must survive a phase ending: the
// real sound path is reached, finds no file in the config directory, and stays
// silent rather than panicking on a nil play function.
func TestAPhaseOnAFreshModelPlaysSilently(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := at(New(testConfig()), start)
	m = press(t, m, "enter")

	_, cmd := tickAt(t, m, start.Add(25*time.Minute+time.Second))

	if got := soundsPlayed(cmd); got != 1 {
		t.Errorf("played %d sounds, want 1 for the phase that ran out", got)
	}
}

// The regression that matters: with the process frozen across a phase boundary,
// the phase that follows must not be handed a fresh full duration.
func TestASuspendDoesNotHandBackExtraTime(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	// An hour passes. The split is focus 0-25m, break 25-30m, focus 30-55m, then
	// a long break ending at 70m. So an hour in, the long break owes ten minutes
	// rather than a fresh fifteen.
	late := start.Add(time.Hour)
	m, cmd := tickAt(t, m, late)

	if m.screen != screenSession {
		t.Fatalf("screen = %v, want the session; the long break still has time on it", m.screen)
	}
	if m.timer.Phase() != timer.PhaseLongBreak {
		t.Fatalf("phase = %v, want the long break", m.timer.Phase())
	}
	if got, want := m.timer.Remaining(late), 10*time.Minute; got != want {
		t.Errorf("Remaining = %v, want %v; a suspend must not restart the break", got, want)
	}

	// A sound per phase that ran out, so a suspend across three boundaries does
	// not go unnoticed. The count is what matters; the phases themselves are
	// covered by the timer's own tests.
	if got := soundsPlayed(cmd); got != 3 {
		t.Errorf("played %d sounds, want one for each phase that ran out", got)
	}
}

func TestResumingAfterASuspendKeepsTheOriginalSchedule(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	// Twenty six minutes in: focus ran out a minute ago, so the break owes four
	// of its five minutes rather than all five.
	late := start.Add(26 * time.Minute)
	m, cmd := tickAt(t, m, late)

	if m.screen != screenSession {
		t.Fatalf("screen = %v, want the session", m.screen)
	}
	if got, want := m.timer.Remaining(late), 4*time.Minute; got != want {
		t.Errorf("Remaining = %v, want %v", got, want)
	}
	if got := soundsPlayed(cmd); got != 1 {
		t.Errorf("played %d sounds, want 1 for the one phase that ran out", got)
	}
}

func TestAPausedTimerIsNotAdvancedByATick(t *testing.T) {
	m := press(t, newTestModel(t), "enter")
	m = press(t, at(m, start.Add(10*time.Minute)), "space")

	// Four hours away. A paused timer must not have ended.
	m, cmd := tickAt(t, m, start.Add(4*time.Hour))

	if m.timer.Phase() != timer.PhaseFocus {
		t.Errorf("phase = %v, want focus; a pause must survive absence", m.timer.Phase())
	}
	if !m.timer.Paused() {
		t.Error("Paused() = false after a late tick, want true")
	}
	if got := soundsPlayed(cmd); got != 0 {
		t.Errorf("played %d sounds, want none while paused", got)
	}
}

func TestTheWindowSizeIsRecorded(t *testing.T) {
	m := send(t, newTestModel(t), tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.width != 120 || m.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", m.width, m.height)
	}
}

// Nothing needs to happen before the first message arrives.
func TestInitAsksForNothingUpFront(t *testing.T) {
	if cmd := newTestModel(t).Init(); cmd != nil {
		t.Error("Init returned a command, want none; the clock only starts once a split is chosen")
	}
}

// The alternate screen is what replaced shelling out to `clear`: it keeps the
// shell's scrollback intact and restores the terminal on exit.
func TestTheViewDrawsOnTheAlternateScreen(t *testing.T) {
	v := newTestModel(t).View()

	if !v.AltScreen {
		t.Error("AltScreen is false, want true")
	}
	if strings.TrimSpace(stripANSI(v.Content)) == "" {
		t.Error("the view has no content")
	}
}

func TestEachScreenRendersItsContent(t *testing.T) {
	picker := newTestModel(t)

	session := press(t, picker, "enter")

	summary := walkToSummary(t, picker, start)

	for _, c := range []struct {
		name  string
		model Model
		want  []string
	}{
		{"picker", picker, []string{"Classic", "Long", "2 × 25 min", "4 × 50 min"}},
		{"session", session, []string{"25:00", "FOCUS", "round 1 of 2", "pause", "skip"}},
		{"summary", summary, []string{"complete", "Classic", "2 rounds", "focused"}},
	} {
		got := stripANSI(c.model.render())
		if strings.TrimSpace(got) == "" {
			t.Errorf("%s rendered nothing", c.name)
			continue
		}
		for _, want := range c.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s screen is missing %q:\n%s", c.name, want, got)
			}
		}
	}
}

func TestTheCountdownFollowsTheClock(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	m = at(m, start.Add(24*time.Minute+30*time.Second))

	if got, want := stripANSI(m.render()), "00:30"; !strings.Contains(got, want) {
		t.Errorf("session screen = %q, want it to show %s", got, want)
	}
}

func TestTheBarFillsAsThePhaseProgresses(t *testing.T) {
	m := press(t, newTestModel(t), "enter")
	width := m.barWidth()

	if got := filledCells(m); got != 0 {
		t.Errorf("filled cells at the start = %d, want 0", got)
	}

	// Halfway through a twenty five minute focus, half the bar is filled. The
	// bar keeps a constant width, so it is the filled cells that change.
	halfway := at(m, start.Add(12*time.Minute+30*time.Second))
	if got, want := filledCells(halfway), width/2; got != want {
		t.Errorf("filled cells halfway = %d, want %d of %d", got, want, width)
	}
}

// filledCells counts the drawn portion of the progress bar.
func filledCells(m Model) int {
	return strings.Count(stripANSI(m.render()), "\u2501")
}

// soundsPlayed runs the commands an update asked for and counts the completion
// sounds they played.
//
// Ticks sleep until the next whole second and never play anything, so the wait
// is bounded by a deadline rather than by every command finishing. Sound
// commands report immediately, so the window is generous for them.
func soundsPlayed(cmd tea.Cmd) int {
	if cmd == nil {
		return 0
	}

	results := make(chan tea.Msg, 64)
	launch := func(c tea.Cmd) { go func() { results <- c() }() }

	// outstanding counts launched commands that have not yet produced a message,
	// so batches get unpacked as they arrive instead of by recursing.
	outstanding := 1
	launch(cmd)

	played := 0
	deadline := time.After(250 * time.Millisecond)

	for outstanding > 0 {
		select {
		case msg := <-results:
			outstanding--

			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, child := range batch {
					outstanding++
					launch(child)
				}
				continue
			}
			if _, ok := msg.(soundPlayedMsg); ok {
				played++
			}

		case <-deadline:
			return played
		}
	}

	return played
}

// stripANSI removes escape sequences so tests can assert on readable text.
func stripANSI(s string) string {
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

// skip advances the model's clock to now and then skips the current phase.
//
// The clock moves first so the skip is handled at the new time. Doing it the
// other way round would apply the previous instant, which quietly dates every
// record a session produces.
func skip(t *testing.T, m Model, now time.Time) Model {
	t.Helper()
	return press(t, at(m, now), "s")
}

// walkToSummary skips through every phase of the split so the model reaches its
// summary, and returns the finished model.
func walkToSummary(t *testing.T, m Model, begin time.Time) Model {
	t.Helper()

	m = at(press(t, m, "enter"), begin)
	for range 20 {
		if m.screen == screenSummary {
			return m
		}
		m = skip(t, m, m.now().Add(90*time.Minute))
	}
	t.Fatal("the split never finished")
	return m
}

// The summary has to state what was achieved, which is not the plan when phases
// were skipped early.
func TestTheSummaryStatesWhatWasFocusedRatherThanThePlan(t *testing.T) {
	m := newTestModel(t)

	m = at(press(t, m, "enter"), start)
	// Abandon the first round two thirds of the way through, then finish the
	// second in full, then the long break.
	m = skip(t, m, start.Add(16*time.Minute+40*time.Second))
	m = skip(t, m, start.Add(40*time.Minute))
	m = skip(t, m, start.Add(80*time.Minute))
	m = skip(t, m, start.Add(95*time.Minute))

	if m.screen != screenSummary {
		t.Fatalf("screen = %v, want the summary", m.screen)
	}

	shown := stripANSI(m.render())
	if !strings.Contains(shown, "2 rounds") {
		t.Errorf("summary does not report the rounds finished:\n%s", shown)
	}
	// 16m40s of the first round plus the whole of the second.
	if !strings.Contains(shown, "41m") {
		t.Errorf("summary does not report the time actually focused:\n%s", shown)
	}
}

// The session screen says which round of how many, so a long split does not look
// like a short one that has been running too long.
func TestTheSessionScreenSaysWhichRoundItIs(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	if shown := stripANSI(m.render()); !strings.Contains(shown, "round 1 of 2") {
		t.Errorf("session does not say which round it is:\n%s", shown)
	}

	// Through the break and into the second round.
	m = skip(t, m, start.Add(25*time.Minute))
	m = skip(t, m, start.Add(30*time.Minute))

	if shown := stripANSI(m.render()); !strings.Contains(shown, "round 2 of 2") {
		t.Errorf("session does not advance the round count:\n%s", shown)
	}
}

// The round count is meaningless over a break, where there is no round in
// progress, so it is left off rather than shown as a number that cannot be right.
func TestTheRoundCountIsLeftOffDuringABreak(t *testing.T) {
	m := skip(t, press(t, newTestModel(t), "enter"), start)

	shown := stripANSI(m.render())
	if !strings.Contains(shown, "BREAK") {
		t.Fatalf("not in a break:\n%s", shown)
	}
	if strings.Contains(shown, "round") {
		t.Errorf("the round count is shown during a break:\n%s", shown)
	}
}

// The picker has to show the rhythm, or a four round split looks identical to a
// single round one.
func TestThePickerShowsTheRhythmOfEachSplit(t *testing.T) {
	m := newTestModel(t)

	shown := stripANSI(m.render())
	if !strings.Contains(shown, "2 × 25 min") {
		t.Errorf("picker does not show the rounds and round length:\n%s", shown)
	}
}

// A window too small to hold the interface must say so, rather than drawing a screen
// whose rows are clipped or whose help line is off the bottom.
//
// The alternative is worse than useless: a picker with its list cut off reads as a
// bug in toki, and a help line the user cannot see is a feature they cannot use.
func TestATooSmallWindowSaysSoInsteadOfDrawingAScreen(t *testing.T) {
	for _, size := range [][2]int{{20, 8}, {26, 24}, {10, 4}, {80, 6}} {
		m := resized(t, newTestModel(t), size[0], size[1])
		out := stripANSI(m.render())

		if !saysTooSmall(out) {
			t.Errorf("at %dx%d no complaint was made:\n%s", size[0], size[1], out)
		}
		if strings.Contains(out, "Choose a split") {
			t.Errorf("at %dx%d the picker was drawn anyway:\n%s", size[0], size[1], out)
		}
	}
}

// The message has to say what is needed, not merely that there is not enough, or the
// user has no way to know how far to resize.
func TestTheComplaintSaysHowMuchRoomIsNeeded(t *testing.T) {
	out := squeeze(stripANSI(resized(t, newTestModel(t), 20, 8).render()))

	for _, want := range []string{"need27wide", "have20"} {
		if !strings.Contains(out, want) {
			t.Errorf("the complaint does not mention %q:\n%s", want, out)
		}
	}
}

// The height the complaint quotes has to be the height the screen actually needs.
//
// This is the whole reason the height is measured from the drawn screen rather than
// being a constant: the picker is a row taller for every split configured, so any
// fixed number is wrong for some config. A wrong number here is not cosmetic — it is
// the instruction the user resizes their window against.
func TestTheComplaintQuotesTheHeightTheScreenActuallyNeeded(t *testing.T) {
	want := rowsNeededAt(t)

	complaint := stripANSI(resized(t, newTestModel(t), 80, 3).render())
	if got := rowsQuotedBy(complaint); got != want {
		t.Errorf("the complaint asks for %d rows, but the screen needs %d:\n%s",
			got, want, complaint)
	}
}

// Growing the window back has to bring the screen back with it.
//
// This is the whole reason for doing the check in render rather than in a one-off
// error path: a terminal is resized constantly, so "too small" is a state the program
// sits in and leaves, not a message it prints once. A user dragging a window corner
// passes through it on the way to any size.
func TestGrowingTheWindowBackBringsTheScreenBack(t *testing.T) {
	m := resized(t, newTestModel(t), 20, 8)
	if !saysTooSmall(stripANSI(m.render())) {
		t.Fatal("the window was not reported as too small")
	}

	m = resized(t, m, 80, 24)
	if out := stripANSI(m.render()); saysTooSmall(out) {
		t.Errorf("the complaint is still showing at 80x24:\n%s", out)
	}
	if !strings.Contains(stripANSI(m.render()), "Choose a split") {
		t.Error("the picker did not come back at 80x24")
	}
}

// A countdown must not be drawn into a window too small to show it. The timer itself
// keeps running, which is right — pausing a session because the user resized would be
// a worse failure — but the session screen is what tells them how long is left, and a
// countdown they cannot see is the one thing a timer must not leave them with.
func TestTheCountdownIsNotDrawnWhereItCannotBeSeen(t *testing.T) {
	m := press(t, newTestModel(t), "enter")
	m = resized(t, m, 20, 8)

	if out := stripANSI(m.render()); strings.Contains(out, "25:00") {
		t.Errorf("the countdown is still drawn where it cannot be seen:\n%s", out)
	}
}

// A short window is as unusable as a narrow one: the picker's list runs off the
// bottom, taking the help line with it.
func TestATooShortWindowIsAlsoRefused(t *testing.T) {
	m := resized(t, newTestModel(t), 80, 10)

	if !saysTooSmall(stripANSI(m.render())) {
		t.Errorf("a window 10 rows tall was accepted:\n%s", stripANSI(m.render()))
	}
}

// Exactly as tall as the screen needs must be enough. An off-by-one either refuses a
// window that fits or accepts one that does not, and neither is visible in a
// screenshot at a normal size.
func TestAWindowExactlyAsTallAsTheScreenNeedsIsAccepted(t *testing.T) {
	rows := rowsNeededAt(t)

	m := resized(t, newTestModel(t), 80, rows)
	if out := stripANSI(m.render()); saysTooSmall(out) {
		t.Errorf("a window of exactly %d rows was refused:\n%s", rows, out)
	}
}

// One row short must be refused, so the boundary is exact rather than inclusive by
// accident.
func TestOneRowBelowWhatTheScreenNeedsIsRefused(t *testing.T) {
	rows := rowsNeededAt(t)

	m := resized(t, newTestModel(t), 80, rows-1)
	if !saysTooSmall(stripANSI(m.render())) {
		t.Error("a window one row short of what the screen needs was accepted")
	}
}

// A window taller than the screen needs must also be accepted, or the check has simply
// been replaced by a floor that refuses perfectly good windows.
func TestATallerWindowThanNeededIsAccepted(t *testing.T) {
	m := resized(t, newTestModel(t), 80, rowsNeededAt(t)+10)

	if out := stripANSI(m.render()); saysTooSmall(out) {
		t.Errorf("a window with room to spare was refused:\n%s", out)
	}
}

// rowsNeededAt is how many rows the picker occupies, counted from the screen itself
// in a window with room to spare.
//
// It is counted rather than read out of the complaint, which would be no test at all:
// the complaint quotes whatever the code decided to say, so asking it and then
// checking that it agrees with itself passes for any number, wrong ones included. The
// row count of a rendered screen is the one thing here that does not come from the code
// under test.
func rowsNeededAt(t *testing.T) int {
	t.Helper()

	roomy := resized(t, newTestModel(t), 80, 200)
	if saysTooSmall(stripANSI(roomy.render())) {
		t.Fatal("a window 80 by 200 was refused, so there is no screen to measure")
	}
	return strings.Count(roomy.render(), "\n") + 1
}

// rowsQuotedBy pulls the "need N tall" figure out of a complaint.
func rowsQuotedBy(out string) int {
	_, rest, found := strings.Cut(out, "need ")
	if !found {
		return 0
	}
	rows, _, _ := strings.Cut(rest, " tall")
	n, err := strconv.Atoi(strings.TrimSpace(rows))
	if err != nil {
		return 0
	}
	return n
}

// resized sends a window size message and returns the model, which is how a terminal
// resize arrives.
func resized(t *testing.T, m Model, width, height int) Model {
	t.Helper()
	return send(t, m, tea.WindowSizeMsg{Width: width, Height: height})
}

// saysTooSmall reports whether a rendered screen is the complaint about the window.
//
// It ignores line breaks, because the complaint is wrapped to fit the very window it
// is complaining about: at ten columns "window too small" spans two lines. A plain
// substring search would then fail on a message that is present and readable, which is
// the same trap as the wrapped warning in the ui tests.
func saysTooSmall(out string) bool {
	return strings.Contains(squeeze(out), "windowtoosmall")
}

// squeeze removes every space and line break from s.
func squeeze(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' {
			return -1
		}
		return r
	}, s)
}

// Finishing a split must leave nothing behind on disk.
//
// toki used to append every finished split to ~/.config/toki/history.json. That
// file was the only thing it ever wrote, so removing it makes the program write
// nothing at all — which is the property worth pinning, because it is the one
// that cannot be undone later. A user who finds toki has been leaving a record of
// every working minute on their machine has no way to undo the finding.
//
// The check walks the whole temp home rather than looking for one filename. A test
// that looked for history.json specifically would keep passing if the file were
// renamed, or if a second file appeared beside it.
//
// It drives the real model rather than a fake, because a fake writes nothing by
// construction: it would pass whether or not the program records anything, which
// is the whole question here.
func TestFinishingASplitLeavesNothingOnDisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// The config directory is created up front, so "wrote nothing" cannot be
	// satisfied by having had nowhere to write in the first place.
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := New(testConfig())
	m.now = func() time.Time { return start }
	m.play = func() {}

	m = at(press(t, m, "enter"), start)
	now := start
	for range 4 {
		now = now.Add(40 * time.Minute)
		m = skip(t, m, now)
	}

	if m.screen != screenSummary {
		t.Fatalf("screen = %v, want the summary, so the split never finished", m.screen)
	}

	var found []string
	err = filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", home, err)
	}

	if len(found) != 0 {
		t.Errorf("finishing a split wrote %d file(s): %v", len(found), found)
	}
}
