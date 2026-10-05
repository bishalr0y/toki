package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/bishalr0y/toki/internal/config"
	"github.com/bishalr0y/toki/internal/timer"
)

// Split is the display model for one row of the picker.
type Split struct {
	Name      string
	FocusMins int
	BreakMins int
	// Cycles is how many focus rounds the split runs and LongBreakMins how long
	// the rest after the last one lasts. The picker shows both, because a four
	// round split and a one round one with the same focus length are different
	// commitments and used to be indistinguishable.
	Cycles        int
	LongBreakMins int
}

// SplitsFrom adapts config entries for display, keeping ui decoupled from the
// config package's own types.
func SplitsFrom(cfg config.Config) []Split {
	out := make([]Split, 0, len(cfg.Timers))
	for _, t := range cfg.Timers {
		out = append(out, Split{
			Name:          t.Name,
			FocusMins:     t.FocusMins,
			BreakMins:     t.BreakMins,
			Cycles:        t.Cycles,
			LongBreakMins: t.LongBreakMins,
		})
	}
	return out
}

// Picker renders the split-selection screen.
type Picker struct {
	Splits []Split
	Cursor int
	Width  int
}

func (p Picker) View() string {
	rows := make([]string, 0, len(p.Splits)+1)

	for i, s := range p.Splits {
		label := p.rowLabel(i, s)

		if i == p.Cursor {
			rows = append(rows, SelectedStyle.Render(label))
			continue
		}
		rows = append(rows, RowStyle.Render(label))
	}

	if len(p.Splits) == 0 {
		rows = append(rows, ErrorStyle.Render("no timer splits configured — see ~/.config/toki/config.yaml"))
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		append([]string{
			Title(),
			"",
			Subtle.Render("Choose a split"),
			"",
		}, rows...)...,
	)
}

// rowLabel lays out one split, dropping to a compact form when the full
// description will not fit rather than letting it run off the edge.
//
// The choice is made by measuring the wide form rather than by comparing the
// terminal against a guessed column width: the name is variable length, so a
// split called "deep work morning" can push the row past the edge at a width
// where a shorter name would have fitted.
func (p Picker) rowLabel(i int, s Split) string {
	cycles := max(s.Cycles, 1)

	// Both row styles pad by two cells on each side, and that padding is rendered
	// outside the label. Measuring the label against the whole terminal therefore
	// let every row run four cells too wide, which is enough to wrap in a band of
	// widths rather than at one: the picker overflowed from about 47 columns to 50
	// and again from 79 to 82, and fitted neatly in between.
	avail := p.Width - rowPadding

	wide := fmt.Sprintf("%d  %-14s  %d × %2d min  ·  %2d min break, %d min long",
		i+1, s.Name, cycles, s.FocusMins, s.BreakMins, s.LongBreakMins)
	if lipgloss.Width(wide) <= avail {
		return wide
	}

	compact := fmt.Sprintf("%d  %s  %d×%d min", i+1, s.Name, cycles, s.FocusMins)
	if lipgloss.Width(compact) <= avail {
		return compact
	}

	// Even the compact form is too wide, which means the name alone is. Trim it
	// so the numbers that make splits comparable survive: a clipped row loses
	// the rhythm, while a shortened name loses only its tail.
	return fmt.Sprintf("%d  %s  %d×%d min", i+1, trim(s.Name, avail-overhead), cycles, s.FocusMins)
}

// rowPadding is the width a picker row spends on RowStyle's and SelectedStyle's
// two cells of padding on each side, which are rendered outside the row's text.
const rowPadding = 4

// overhead is the width a row spends on everything except the split's name.
const overhead = len("1  name  4×50 min")

// trim shortens s to at most width cells, marking that it has been cut.
func trim(s string, width int) string {
	if width <= 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if width <= 2 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

// Session renders the live countdown screen.
type Session struct {
	SplitName string
	// Phase is the timer's own type rather than its rendered text, so a screen
	// cannot come to a different conclusion about a phase from another one.
	Phase timer.Phase
	// Round says which focus round is running, out of how many the split runs.
	// Blank on a break, where there is no round to count.
	Round       string
	Paused      bool
	Remaining   string
	ElapsedFrac float64 // 0 at the start of the phase, 1 when it is over
	BarWidth    int
	Warning     string
	Width       int
}

func (s Session) View() string {
	label := LabelFor(s.Phase)

	barStyle := FocusBar
	if isRest(s.Phase) {
		barStyle = BreakBar
	}

	// The name is trimmed rather than trusted to fit: it comes from the config, so
	// its length is the user's choice and a long one must not push the screen past
	// the edge. A screen told nothing about the width is not trimmed, since there is
	// nothing to fit inside.
	body := []string{
		fitted(s.Width, Subtle, "split: "+s.SplitName),
		"",
		label,
	}

	// Only while working: the round count would be meaningless over a break.
	if s.Round != "" && s.Phase == timer.PhaseFocus {
		body = append(body, Subtle.Render(s.Round))
	}

	body = append(body,
		"",
		Countdown.Render(s.Remaining),
		"",
		Bar(s.ElapsedFrac, s.BarWidth, barStyle),
	)

	if s.Paused {
		body = append(body, "", PausedLabel.Render("⏸  paused"))
	}
	if s.Warning != "" {
		body = append(body, "", warn(s.Warning, s.Width))
	}

	return lipgloss.JoinVertical(lipgloss.Center, body...)
}

// defaultWidth is the width assumed when a screen has not been told one.
//
// The zero value of Width has to render something readable rather than something
// technically correct: wrapping to zero cells puts the warning one character per
// line, which is unreadable and looks like a crash. The real program always passes a
// width, so this only affects a screen built without one — chiefly a test.
const defaultWidth = 80

// fitted renders text trimmed to width, or untouched when no width was given.
//
// Trimming is right for a single short line of the user's own text, such as a split
// name: the end of it is not the point, so an ellipsis costs nothing. It is wrong
// for a warning, which is why that wraps instead.
func fitted(width int, style lipgloss.Style, text string) string {
	if width < 1 {
		return style.Render(text)
	}
	return style.Render(trim(text, width))
}

// warn renders a failure message, wrapped to the terminal and then capped.
//
// Wrapping rather than truncating, because the end of one of these messages is its
// point: "permission denied" and the path it applies to are both the answer, and a
// clipped line that hid them would leave the reader with a warning they cannot act
// on. The cap is still applied afterwards so that a single unbroken word longer
// than the terminal — a path, a filename — cannot push the edge out on its own.
func warn(message string, width int) string {
	if width < 1 {
		width = defaultWidth
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(ErrorStyle.Width(width).Render("⚠  " + message))
}

// Summary renders the screen shown once a split is complete.
type Summary struct {
	SplitName string
	// Rounds is how many focus rounds finished and Focus how long was actually
	// worked. These are the achievement rather than the plan, because a phase
	// skipped early means less time was focused than the split calls for.
	Rounds int
	Focus  string
	// Today is the running total for the day, blank when there is none to report.
	Today string

	// Sent and Failed count the desktop notifications that were actually
	// attempted, so a session ended by skipping them does not claim they failed.
	Sent   int
	Failed int

	// Warning reports anything that went wrong on the way here, such as history
	// that could not be written.
	Warning string

	Width int
}

func (s Summary) View() string {
	body := []string{
		"",
		SuccessStyle.Render("✓  split complete"),
		"",
		// Trimmed for the same reason as the session screen's name: it is the user's
		// text and its length is theirs to choose.
		fitted(s.Width, TextStyle, s.SplitName),
		TextStyle.Render(fmt.Sprintf("%d %s  ·  %s focused",
			s.Rounds, Plural(s.Rounds, "round", "rounds"), s.Focus)),
	}

	if s.Today != "" {
		body = append(body, Subtle.Render(s.Today))
	}
	if notice := s.notice(); notice != "" {
		body = append(body, "", notice)
	}
	if s.Warning != "" {
		body = append(body, "", warn(s.Warning, s.Width))
	}

	return lipgloss.JoinVertical(lipgloss.Center, body...)
}

// notice reports what happened to the notifications, and says nothing at all
// when none were ever attempted.
func (s Summary) notice() string {
	switch {
	case s.Failed > 0:
		return ErrorStyle.Render(fmt.Sprintf("%d notification(s) could not be sent", s.Failed))
	case s.Sent > 0:
		return Subtle.Render("notifications sent")
	default:
		return ""
	}
}

// Plural picks between a singular and a plural noun for a count.
//
// Exported because both the summary and the day's running total count things, and
// "1 splits" is the sort of detail that makes a screen look unfinished.
func Plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// separator sits between key hints.
const separator = "   ·   "

// Help renders the key hints for a screen, wrapped to fit the terminal.
//
// Wrapping matters on a narrow window: a hint that runs off the edge is simply
// lost, so the keys that do not fit would be invisible.
//
// Lines are broken only between entries. Wrapping on spaces alone would split
// the separator, leaving a lone "·" at the start of a line.
//
// The result is deliberately left unpadded so that whatever centres it also
// centres these lines; padding them to the full width first would leave the
// centring nothing to do.
func Help(width int, entries ...[2]string) string {
	rendered := make([]string, 0, len(entries))
	for _, e := range entries {
		rendered = append(rendered, KeyCap.Render(e[0])+" "+KeyStyle.Render(e[1]))
	}

	if width <= 0 {
		return strings.Join(rendered, Subtle.Render(separator))
	}

	sepWidth := lipgloss.Width(separator)

	var lines []string
	line := ""

	for _, entry := range rendered {
		switch {
		case line == "":
			line = entry
		case lipgloss.Width(line)+sepWidth+lipgloss.Width(entry) <= width:
			line += Subtle.Render(separator) + entry
		default:
			lines = append(lines, line)
			line = entry
		}
	}

	if line != "" {
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// MinimumWidth is the narrowest terminal the interface can be drawn in.
//
// It is the width of the longest fixed string any screen must show — "4 rounds  ·
// 1h 40m focused" on the summary, at 27 cells. Below that, something has to wrap,
// and a wrapped line has its remainder drawn over the line beneath it, so the screen
// stops lining up rather than merely looking tight.
//
// There is no matching height constant, because the height a screen needs is not a
// property of the terminal but of the screen: the picker is a row taller per split
// configured, and a long warning adds rows to whichever screen carries it. Callers
// measure the screen they drew and pass the result to TooSmallTall.
const MinimumWidth = 27

// TooSmall renders the message shown when the terminal is too narrow to hold the
// interface.
//
// It is drawn instead of a screen rather than alongside it, because the alternative
// is a picker whose rows are clipped mid-list — which reads as a bug in toki — or a
// help line the user cannot see, which is a feature they cannot use.
func TooSmall(width int) string {
	return complaint(width, fmt.Sprintf("need %d wide, have %d", MinimumWidth, width))
}

// TooSmallTall is TooSmall for a window that is wide enough but not tall enough, and
// is told how many rows the screen would have needed so that the message can say so.
//
// The number is measured rather than declared because a fixed minimum would be a lie
// in one direction or the other: it would clip the split that made the list taller,
// or refuse a window with room to spare. A user who reads "need 16, have 14" can act
// on it; a user who reads a rounded-off guess cannot.
//
// It says nothing about the width, which is not what is wrong here, and quoting it
// anyway would send the user off to widen a window that is already wide enough.
func TooSmallTall(width, height, need int) string {
	return complaint(width, fmt.Sprintf("need %d tall, have %d", need, height))
}

// complaint renders the message for a window that cannot hold the interface, with
// detail explaining by how much.
func complaint(width int, detail string) string {
	body := []string{
		ErrorStyle.Render("window too small"),
		"",
		// Wrapped rather than clipped, because the sizes are the whole message: a
		// window too narrow to hold them is exactly the window this is drawn in.
		Subtle.Render(detail),
	}

	// Left-aligned rather than centred, and capped at the window that asked:
	// centring pads every line to the widest one, so in a window too small for that
	// a message about being too small ends up wider than the window.
	joined := lipgloss.JoinVertical(lipgloss.Left, body...)
	return lipgloss.NewStyle().MaxWidth(max(width, 1)).Render(
		Subtle.Width(max(width, 1)).Render(joined),
	)
}
