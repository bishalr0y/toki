package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/bishalr0y/toki/internal/config"
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

	wide := fmt.Sprintf("%d  %-14s  %d × %2d min  ·  %2d min break, %d min long",
		i+1, s.Name, cycles, s.FocusMins, s.BreakMins, s.LongBreakMins)
	if lipgloss.Width(wide) <= p.Width {
		return wide
	}

	compact := fmt.Sprintf("%d  %s  %d×%d min", i+1, s.Name, cycles, s.FocusMins)
	if lipgloss.Width(compact) <= p.Width {
		return compact
	}

	// Even the compact form is too wide, which means the name alone is. Trim it
	// so the numbers that make splits comparable survive: a clipped row loses
	// the rhythm, while a shortened name loses only its tail.
	return fmt.Sprintf("%d  %s  %d×%d min", i+1, trim(s.Name, p.Width-overhead), cycles, s.FocusMins)
}

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
	Phase     string
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
	if s.Phase == "BREAK" || s.Phase == "LONG BREAK" {
		barStyle = BreakBar
	}

	body := []string{
		Subtle.Render("split: " + s.SplitName),
		"",
		label,
	}

	// Only while working: the round count would be meaningless over a break.
	if s.Round != "" && s.Phase == "FOCUS" {
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
		body = append(body, "", ErrorStyle.Render("⚠  "+s.Warning))
	}

	return lipgloss.JoinVertical(lipgloss.Center, body...)
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
		TextStyle.Render(s.SplitName),
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
		body = append(body, "", ErrorStyle.Render("⚠  "+s.Warning))
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
