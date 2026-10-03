package app

import (
	"errors"
	"sort"
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
		{Name: "Classic", FocusMins: 25, BreakMins: 5},
		{Name: "Long", FocusMins: 50, BreakMins: 10},
	}}
}

// newTestModel returns a model whose clock is frozen at start and whose
// notifier is a stub, so nothing pops up on the developer's desktop.
func newTestModel(t *testing.T) Model {
	t.Helper()

	m := New(testConfig())
	m.now = func() time.Time { return start }
	m.notify = func(string) error { return nil }

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

func TestSkippingTheLastPhaseShowsTheSummary(t *testing.T) {
	m := press(t, press(t, press(t, newTestModel(t), "enter"), "s"), "s")

	if m.screen != screenSummary {
		t.Errorf("screen = %v, want the summary", m.screen)
	}
	if !m.timer.Finished() {
		t.Error("Finished() = false, want true")
	}
}

func TestEscapeGoesBackToThePicker(t *testing.T) {
	m := press(t, press(t, newTestModel(t), "enter"), "esc")

	if m.screen != screenPicker {
		t.Errorf("screen = %v, want the picker", m.screen)
	}
}

// Leaving and starting again must begin a clean session, with the full focus
// duration and no warning left over from the abandoned one.
func TestStartingAgainAfterLeavingCarriesNoStaleState(t *testing.T) {
	m := press(t, newTestModel(t), "enter")
	m = press(t, m, "s") // into the break
	m = send(t, m, notifyMsg{phase: timer.PhaseFocus, err: errors.New("boom")})
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
	if m.warning != "" {
		t.Errorf("warning = %q, want it cleared for the new session", m.warning)
	}
}

func TestTheSummaryGoesBackToThePicker(t *testing.T) {
	m := press(t, press(t, press(t, press(t, newTestModel(t), "enter"), "s"), "s"), "enter")

	if m.screen != screenPicker {
		t.Errorf("screen = %v, want the picker", m.screen)
	}
}

func TestAPhaseThatRunsOutNotifiesAndAdvances(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	// One second past the focus deadline.
	m, cmd := tickAt(t, m, start.Add(25*time.Minute+time.Second))

	if m.timer.Phase() != timer.PhaseBreak {
		t.Errorf("phase = %v, want break", m.timer.Phase())
	}
	if got, want := reported(cmd), []string{"FOCUS"}; !equal(got, want) {
		t.Errorf("notified %v, want %v", got, want)
	}
}

// The regression that matters: with the process frozen across a phase boundary,
// the break must not be handed a fresh full duration.
func TestASuspendDoesNotHandBackExtraTime(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	// Half an hour passes: both the focus and the break ran out.
	m, cmd := tickAt(t, m, start.Add(30*time.Minute))

	if m.screen != screenSummary {
		t.Errorf("screen = %v, want the summary; a suspend must not restart the break", m.screen)
	}
	if got, want := reported(cmd), []string{"BREAK", "FOCUS"}; !equal(got, want) {
		t.Errorf("notified %v, want %v; both phases ran out", got, want)
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
	if got, want := reported(cmd), []string{"FOCUS"}; !equal(got, want) {
		t.Errorf("notified %v, want %v", got, want)
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
	if got := reported(cmd); len(got) != 0 {
		t.Errorf("notified %v, want nothing while paused", got)
	}
}

func TestANotificationFailureReachesTheScreen(t *testing.T) {
	m := press(t, newTestModel(t), "enter")

	m = send(t, m, notifyMsg{phase: timer.PhaseFocus, err: errors.New("no notification daemon")})

	if !strings.Contains(m.warning, "no notification daemon") {
		t.Errorf("warning = %q, want it to name the failure", m.warning)
	}
	if m.failed != 1 || m.sent != 0 {
		t.Errorf("tallies = %d sent, %d failed; want 0 sent, 1 failed", m.sent, m.failed)
	}
	if got := stripANSI(m.render()); !strings.Contains(got, "no notification daemon") {
		t.Error("the rendered screen does not mention the failure")
	}

	// A later success clears it.
	m = send(t, m, notifyMsg{phase: timer.PhaseBreak})
	if m.warning != "" {
		t.Errorf("warning = %q after a success, want it cleared", m.warning)
	}
	if m.sent != 1 {
		t.Errorf("sent = %d, want 1", m.sent)
	}
}

// A session ended by skipping never attempted a notification, so the summary
// must not claim they failed or that they were unavailable.
func TestSkippingThroughToTheSummaryClaimsNothingAboutNotifications(t *testing.T) {
	m := press(t, press(t, press(t, newTestModel(t), "enter"), "s"), "s")

	got := stripANSI(m.render())
	if strings.Contains(got, "notification") {
		t.Errorf("summary mentions notifications though none were attempted:\n%s", got)
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

	summary := press(t, press(t, session, "s"), "s")

	for _, c := range []struct {
		name  string
		model Model
		want  []string
	}{
		{"picker", picker, []string{"Classic", "Long", "25", "50"}},
		{"session", session, []string{"25:00", "FOCUS", "pause", "skip"}},
		{"summary", summary, []string{"complete", "25 min focus", "5 min break"}},
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

// reported runs the commands an update asked for and returns the phases they
// notified, sorted.
//
// Ticks sleep until the next whole second and never report anything, so the
// wait is bounded by a deadline rather than by every command finishing.
// Notification commands report immediately, so the window is generous for them.
func reported(cmd tea.Cmd) []string {
	if cmd == nil {
		return nil
	}

	results := make(chan tea.Msg, 64)
	launch := func(c tea.Cmd) { go func() { results <- c() }() }

	// outstanding counts launched commands that have not yet produced a message,
	// so batches get unpacked as they arrive instead of by recursing.
	outstanding := 1
	launch(cmd)

	var found []string
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
			if n, ok := msg.(notifyMsg); ok && n.err == nil {
				found = append(found, n.phase.String())
			}

		case <-deadline:
			sort.Strings(found)
			return found
		}
	}

	sort.Strings(found)
	return found
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
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
