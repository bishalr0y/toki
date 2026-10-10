// Package app wires the terminal UI together.
//
// All decision making lives in Model.Update, which is an ordinary function on a
// value type: screens are driven by calling it with a message and reading the
// state it returns, so the whole application is testable without a terminal.
package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bishalr0y/toki/internal/config"
	"github.com/bishalr0y/toki/internal/history"
	"github.com/bishalr0y/toki/internal/sound"
	"github.com/bishalr0y/toki/internal/timer"
	"github.com/bishalr0y/toki/internal/ui"
)

// screen is which of toki's screens is showing.
type screen int

const (
	screenPicker screen = iota
	screenSession
	screenSummary
)

// tickMsg carries the instant a scheduled redraw came due.
//
// The instant travels in the message rather than being read from the clock
// inside Update, so tests can drive time deliberately.
type tickMsg time.Time

// soundPlayedMsg reports that a completion sound was attempted. It carries no
// outcome — playback is best-effort — but routing it back through Update keeps
// the sound tied to the update loop rather than firing off on its own.
type soundPlayedMsg struct{}

// recordedMsg reports that a finished split was written to the history. It carries
// no outcome for the same reason soundPlayedMsg does not: the write is best effort,
// and Update does not need to know whether it worked to carry on.
type recordedMsg struct{}

// Model is the entire application state.
type Model struct {
	splits []ui.Split
	cursor int
	screen screen

	// today is the day's total, read once at startup and refreshed when a split
	// finishes. Only the picker and the session screen show it; --stats and
	// --history are printed from the history package directly.
	today history.Totals

	timer timer.Timer
	split timer.Split
	name  string

	// focused and rounds are kept as they were at the moment the split ended,
	// because once the timer goes back to PhaseIdle it no longer knows.
	focused time.Duration
	rounds  int

	width  int
	height int

	// now is indirected so tests can anchor a session to a fixed instant
	// instead of the wall clock.
	now func() time.Time
	// play is how a finished phase announces itself, indirected so tests do not
	// reach for the speakers and so a machine with no sound stays quiet.
	play func()
}

// New builds the initial model for a set of configured splits.
func New(cfg config.Config) Model {
	return Model{
		splits: ui.SplitsFrom(cfg),
		screen: screenPicker,
		// Sensible defaults so the very first frame, drawn before any
		// WindowSizeMsg arrives, is not degenerate.
		width:  80,
		height: 24,
		now:    time.Now,
		play:   playCompletionSound,
	}
}

// playCompletionSound plays the user's completion sound, if they have one.
//
// The config directory is resolved here rather than once at construction so the
// sound is picked up per play, and a machine whose home directory cannot be
// found is simply silent rather than an error anyone has to handle.
func playCompletionSound() {
	dir, err := config.Dir()
	if err != nil {
		return
	}
	sound.Play(dir)
}

// Init satisfies tea.Model. Nothing needs to happen before the first message.
func (m Model) Init() tea.Cmd { return nil }

// LoadStats fills in the totals the session and stats screens show.
//
// It reads once, at startup, and the numbers it produces then only change when a
// split finishes. Reading on every tick would cost a file scan several times a
// second to produce an answer that cannot have moved.
//
// A history that cannot be read leaves the totals at zero rather than stopping
// toki: the screens then show nothing, which is the same as having no history
// yet, and a timer that will not start over a statistics file is worse than one
// that shows a zero.
func (m Model) LoadStats() Model {
	totals, err := history.Today(m.now())
	if err == nil {
		m.today = totals
	}

	return m
}

// refreshToday brings today's total up to date, after a split has been recorded.
//
// It is called when a split finishes rather than on every tick, because that is
// the only moment the number can change.
func (m Model) refreshToday() Model {
	if totals, err := history.Today(m.now()); err == nil {
		m.today = totals
	}
	return m
}

// todayRunning is today's total including the split in progress.
//
// Including it is what makes the line worth watching. A total that only counted
// finished splits would sit unchanged for the whole 25 minutes it is on screen,
// and would then jump; one that moves as rounds complete answers "how am I doing"
// while the answer is still forming, and lands on exactly the figure `--stats`
// shows once the split is recorded.
//
// The figures come from the timer rather than from the record, so nothing is
// written twice and the summary cannot disagree with the line above the countdown.
func (m Model) todayRunning(now time.Time) history.Totals {
	t := m.today

	// Shown only while working. The round count in the line is the day's, but
	// during a break there is no round in progress, and a number about rounds
	// appearing over a rest is the same confusion the session screen already
	// avoids by leaving its own round label out there.
	if !m.timer.Started() || m.timer.Phase() != timer.PhaseFocus {
		return t
	}

	t.Focused += m.timer.Focused(now)
	t.Rounds += m.timer.Rounds()
	t.Splits++

	return t
}

// Update satisfies tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		return m.onTick(time.Time(msg))

	case tea.KeyPressMsg:
		return m.onKey(msg.String(), m.now())

	case recordedMsg:
		// Re-read rather than adding the split to the total by hand. The split was
		// just written, so reading it back is a few hundred lines, and it means the
		// number on screen is the one the file actually holds rather than a running
		// total assembled in memory — which is what would drift if a write ever
		// failed, silently and invisibly.
		return m.refreshToday(), nil
	}

	return m, nil
}

func (m Model) onTick(now time.Time) (tea.Model, tea.Cmd) {
	if m.screen != screenSession {
		return m, nil
	}

	// Nothing has run out yet, so just line up the next redraw.
	if !m.timer.Expired(now) {
		return m, m.tickCmd(now)
	}

	// One sound per phase that ended, which may be more than one if the process
	// was suspended across a boundary. Playback is best-effort and never blocks
	// the timer.
	ended := m.timer.Advance(now)
	cmds := make([]tea.Cmd, 0, len(ended)+1)
	for range ended {
		cmds = append(cmds, m.playSoundCmd())
	}

	if m.timer.Finished() {
		var r history.Record
		m, r = m.complete(now)
		cmds = append(cmds, recordCmd(r))
	} else {
		cmds = append(cmds, m.tickCmd(now))
	}

	return m, tea.Batch(cmds...)
}

// complete marks the split finished: it keeps what was achieved, moves to the
// summary, and reports the record to write.
//
// The record is returned rather than written here so that complete stays a pure
// function of the model, which is what makes it testable without a filesystem. The
// caller decides what to do with it — and it is told to decide, because writing is
// the only thing in this function that can fail.
//
// An abandoned split still records. Esc during a session goes back to the picker
// without calling this, so reaching complete means the split ran its course; how
// many rounds it managed is a fact about the session rather than a reason to
// discard it.
func (m Model) complete(now time.Time) (Model, history.Record) {
	m.focused = m.timer.Focused(now)
	m.rounds = m.timer.Rounds()
	m.screen = screenSummary

	return m, history.Record{
		Name:    m.name,
		Focused: m.focused,
		Rounds:  m.rounds,
		EndedAt: now,
	}
}

// recordCmd writes a finished split to the history.
//
// Best effort, exactly as sound.Play is. A split that ran for an hour has already
// happened; failing to note it down is not worth interrupting the user over, and
// there is no failure this could produce that the timer cannot keep running
// through. The error is dropped rather than logged because there is nowhere to log
// it — writing to the terminal would corrupt the screen, and toki keeps no log file
// by design.
func recordCmd(r history.Record) tea.Cmd {
	return func() tea.Msg {
		_ = history.Append(r)
		return recordedMsg{}
	}
}

func (m Model) onKey(k string, now time.Time) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenPicker:
		return m.onPickerKey(k, now)
	case screenSession:
		return m.onSessionKey(k, now)
	case screenSummary:
		if k == keyEnter || isQuit(k) {
			return m.toPicker(), nil
		}

		// The two stats screens are read-only, so any key that is not quitting or
		// asking to leave goes back the same way. There is nothing on them to edit.
		switch {
		case isQuit(k):
			return m, tea.Quit
		case k == keyEnter || k == keyEsc:
			return m.toPicker(), nil
		}
	}
	return m, nil
}

func (m Model) onPickerKey(k string, now time.Time) (tea.Model, tea.Cmd) {
	switch {
	case isQuit(k):
		return m, tea.Quit
	case isUp(k):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case isDown(k):
		if m.cursor < len(m.splits)-1 {
			m.cursor++
		}
		return m, nil
	case k == keyEnter:
		return m.start(now)
	}

	if idx, ok := digitIndex(k, len(m.splits)); ok {
		m.cursor = idx
		return m.start(now)
	}
	return m, nil
}

func (m Model) onSessionKey(k string, now time.Time) (tea.Model, tea.Cmd) {
	switch {
	case isQuit(k):
		return m, tea.Quit
	case k == keyEsc:
		return m.toPicker(), nil
	case k == keySpace:
		if m.timer.Paused() {
			m.timer.Resume(now)
		} else {
			m.timer.Pause(now)
		}
		return m, nil
	case k == keySkip:
		return m.skipPhase(now)
	}
	return m, nil
}

// start begins a session for the highlighted split.
func (m Model) start(now time.Time) (tea.Model, tea.Cmd) {
	if len(m.splits) == 0 {
		return m, nil
	}

	split := m.splits[m.cursor]
	m.split = timer.Split{
		Focus:     time.Duration(split.FocusMins) * time.Minute,
		Break:     time.Duration(split.BreakMins) * time.Minute,
		LongBreak: time.Duration(split.LongBreakMins) * time.Minute,
		Cycles:    split.Cycles,
	}
	m.name = split.Name
	m.timer = timer.New(m.split, now)
	m.focused, m.rounds = 0, 0
	m.screen = screenSession

	return m, m.tickCmd(now)
}

// skipPhase abandons the current phase early.
func (m Model) skipPhase(now time.Time) (tea.Model, tea.Cmd) {
	if m.timer.Finished() {
		return m, nil
	}

	m.timer.Skip(now)
	if m.timer.Finished() {
		var r history.Record
		m, r = m.complete(now)
		return m, recordCmd(r)
	}
	return m, m.tickCmd(now)
}

func (m Model) toPicker() Model {
	m.screen = screenPicker
	m.timer = timer.Timer{}
	return m
}

// tickCmd schedules the next redraw on the next whole second, so the countdown
// stays in step with the wall clock.
func (m Model) tickCmd(now time.Time) tea.Cmd {
	return tea.Tick(timer.NextTick(now), func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// View satisfies tea.Model.
func (m Model) View() tea.View {
	// The screen is drawn as one block and then placed in the middle of the
	// terminal, across and down, rather than left to hug the top-left corner.
	//
	// The placing happens here rather than in render because render's output is
	// what the "too small" check measures: padding the screen out to the whole
	// window first would leave that check nothing to notice, and a countdown
	// clipped off the bottom would be accepted as fitting.
	v := tea.NewView(lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		m.render(),
	))
	// Draw on the alternate screen so the interface never scrolls the user's
	// shell history, and so quitting restores the terminal as it was found.
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	// A window too small to hold the interface gets a complaint rather than a screen.
	// Drawing anyway is worse than it sounds: the picker's list runs off the bottom,
	// which reads as a bug in toki, and a countdown on screen that cannot be seen is
	// the one thing a timer must not do.
	//
	// This is checked here, on every frame, rather than on the way in, because a
	// terminal is resized constantly — a user dragging a window corner passes
	// through this size on the way to any other, so "too small" is a state the
	// program sits in and then leaves, not a message it prints once.
	if m.width < ui.MinimumWidth {
		return ui.TooSmall(m.width)
	}

	var body, help string

	switch m.screen {
	case screenSession:
		body = ui.Session{
			SplitName:   m.name,
			Phase:       m.timer.Phase(),
			Round:       roundLabel(m.timer),
			Paused:      m.timer.Paused(),
			Remaining:   timer.Format(m.timer.Remaining(m.now())),
			ElapsedFrac: m.timer.Progress(m.now()),
			BarWidth:    m.barWidth(),
			Width:       m.width,
			Today:       ui.TodayLine(m.todayRunning(m.now())),
		}.View()
		help = ui.Help(m.width,
			[2]string{"space", "pause"},
			[2]string{"s", "skip"},
			[2]string{"esc", "back"},
			[2]string{"q", "quit"},
		)

	case screenSummary:
		body = ui.Summary{
			SplitName: m.name,
			Rounds:    m.rounds,
			Focus:     timer.FormatTotal(m.focused),
			Width:     m.width,
		}.View()
		help = ui.Help(m.width, [2]string{"enter", "back"}, [2]string{"q", "quit"})

	default:
		body = ui.Picker{
			Splits: m.splits,
			Cursor: m.cursor,
			Width:  m.width,
			Today:  ui.TodayLine(m.today),
		}.View()
		help = ui.Help(m.width,
			[2]string{"↑/↓", "move"},
			[2]string{"enter", "start"},
			[2]string{"q", "quit"},
		)
	}

	out := lipgloss.JoinVertical(lipgloss.Center, body, "", help)

	// The height is checked against what was just drawn rather than against a
	// constant, because only the drawn screen knows how tall it turned out to be:
	// the picker grows by a row per split. A constant floor would either clip the
	// split that pushes it over or refuse a window that happens to be roomy enough,
	// and both would make the "have X by Y" in the complaint a lie.
	if rows := strings.Count(out, "\n") + 1; rows > m.height {
		return ui.TooSmallTall(m.width, m.height, rows)
	}

	return out
}

// barWidth is how wide the progress bar is drawn.
//
// The floor is deliberately lower than it looks: a bar wider than the terminal
// wraps, and the wrapped remainder is drawn over the line below it, so on a narrow
// window the bar would land on top of the help text for the rest of the session.
// The upper bound keeps the bar a reasonable shape on a wide window rather than
// letting it run the full width.
func (m Model) barWidth() int {
	return min(44, max(m.width-10, 4))
}

// playSoundCmd plays the completion sound off the update loop, so a player that
// takes a moment to start cannot hold up the countdown.
//
// It reports an outcome rather than playing silently in the command, because that
// keeps the side effect inside the update loop: a test can run the commands an
// update asked for and count how many sounds a phase boundary produced.
func (m Model) playSoundCmd() tea.Cmd {
	play := m.play
	return func() tea.Msg {
		play()
		return soundPlayedMsg{}
	}
}

// roundLabel says which focus round is running, out of how many the split runs.
//
// A four round split and a one round one look identical on screen otherwise, and
// there is no way to tell a long session from a short one running too long.
// Rounds finished plus one is the round in progress, and the split's own cycle
// count is the total, so the numbers cannot drift apart.
func roundLabel(tr timer.Timer) string {
	cycles := max(tr.Cycles(), 1)
	return fmt.Sprintf("round %d of %d", min(tr.Rounds()+1, cycles), cycles)
}
