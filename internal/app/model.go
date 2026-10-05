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
	"github.com/bishalr0y/toki/internal/notify"
	"github.com/bishalr0y/toki/internal/timer"
	"github.com/bishalr0y/toki/internal/ui"
)

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

// notifyMsg reports the outcome of a desktop notification. Sending it is a
// tea.Cmd, so a slow or hanging notifier cannot freeze the interface.
type notifyMsg struct {
	phase timer.Phase
	err   error
}

// Model is the entire application state.
type Model struct {
	splits []ui.Split
	cursor int
	screen screen

	timer timer.Timer
	split timer.Split
	name  string

	// focused and rounds are kept as they were at the moment the split ended,
	// because once the timer goes back to PhaseIdle it no longer knows.
	focused time.Duration
	rounds  int

	// sent and failed count the notifications actually attempted, so the
	// summary can report on them without claiming a failure that never happened.
	sent   int
	failed int

	warning string

	width  int
	height int

	// notify is indirected so tests can exercise the notification path without
	// firing real desktop notifications.
	notify func(string) error

	// now is indirected so tests can anchor a session to a fixed instant
	// instead of the wall clock.
	now func() time.Time
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
		notify: notify.Notify,
		now:    time.Now,
	}
}

// Init satisfies tea.Model. Nothing needs to happen before the first message.
func (m Model) Init() tea.Cmd { return nil }

// Update satisfies tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		return m.onTick(time.Time(msg))

	case notifyMsg:
		return m.onNotify(msg), nil

	case tea.KeyPressMsg:
		return m.onKey(msg.String(), m.now())
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

	// One notification per phase that ended, which may be more than one if the
	// process was suspended across a boundary.
	ended := m.timer.Advance(now)
	cmds := make([]tea.Cmd, 0, len(ended)+1)
	for _, phase := range ended {
		cmds = append(cmds, m.notifyCmd(phase))
	}

	if m.timer.Finished() {
		m = m.complete(now)
	} else {
		cmds = append(cmds, m.tickCmd(now))
	}

	return m, tea.Batch(cmds...)
}

// complete marks the split finished: it keeps what was achieved and moves to the
// summary.
//
// The figures stay in the model rather than being written anywhere, so there is
// nothing here that can fail and no warning to give. The summary reports the
// session that just ran, not a day of sessions.
func (m Model) complete(now time.Time) Model {
	m.focused = m.timer.Focused(now)
	m.rounds = m.timer.Rounds()
	m.screen = screenSummary

	return m
}

func (m Model) onNotify(msg notifyMsg) Model {
	if msg.err != nil {
		m.failed++
		m.warning = fmt.Sprintf("desktop notification failed: %v", msg.err)
		return m
	}

	m.sent++
	m.warning = ""
	return m
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
	m.warning = ""
	m.sent, m.failed = 0, 0

	return m, m.tickCmd(now)
}

// skipPhase abandons the current phase early.
func (m Model) skipPhase(now time.Time) (tea.Model, tea.Cmd) {
	if m.timer.Finished() {
		return m, nil
	}

	m.timer.Skip(now)
	if m.timer.Finished() {
		return m.complete(now), nil
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

// notifyCmd sends a desktop notification off the update loop, so a slow or
// hanging notifier cannot freeze the interface, and reports the outcome back as
// a message. Failures are surfaced on screen rather than discarded.
func (m Model) notifyCmd(phase timer.Phase) tea.Cmd {
	send := m.notify
	name := phase.String()

	return func() tea.Msg {
		return notifyMsg{phase: phase, err: send(name)}
	}
}

// View satisfies tea.Model.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
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
			Warning:     m.warning,
			Width:       m.width,
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
			Sent:      m.sent,
			Failed:    m.failed,
			Warning:   m.warning,
			Width:     m.width,
		}.View()
		help = ui.Help(m.width, [2]string{"enter", "back"}, [2]string{"q", "quit"})

	default:
		body = ui.Picker{Splits: m.splits, Cursor: m.cursor, Width: m.width}.View()
		help = ui.Help(m.width,
			[2]string{"↑/↓", "move"},
			[2]string{"enter", "start"},
			[2]string{"q", "quit"},
		)
	}

	out := lipgloss.JoinVertical(lipgloss.Center, body, "", help)

	// The height is checked against what was just drawn rather than against a
	// constant, because only the drawn screen knows how tall it turned out to be:
	// the picker grows by a row per split, and a long warning adds rows to whichever
	// screen it appears on. A constant floor would either clip the split that pushes
	// it over or refuse a window that happens to be roomy enough, and both would make
	// the "have X by Y" in the complaint a lie.
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
