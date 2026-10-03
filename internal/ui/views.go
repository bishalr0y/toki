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
}

// SplitsFrom adapts config entries for display, keeping ui decoupled from the
// config package's own types.
func SplitsFrom(cfg config.Config) []Split {
	out := make([]Split, 0, len(cfg.Timers))
	for _, t := range cfg.Timers {
		out = append(out, Split{Name: t.Name, FocusMins: t.FocusMins, BreakMins: t.BreakMins})
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

// rowLabel lays out one split, dropping to a compact form when the terminal is
// too narrow for the full description rather than letting it run off the edge.
func (p Picker) rowLabel(i int, s Split) string {
	if p.Width < wideRow {
		return fmt.Sprintf("%d  %s  %d/%d min", i+1, s.Name, s.FocusMins, s.BreakMins)
	}
	return fmt.Sprintf("%d  %-14s  %3d min focus  %2d min break", i+1, s.Name, s.FocusMins, s.BreakMins)
}

// wideRow is the narrowest width at which the full row description fits.
const wideRow = 46

// Session renders the live countdown screen.
type Session struct {
	SplitName   string
	Phase       string
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
	if s.Phase == "BREAK" {
		barStyle = BreakBar
	}

	body := []string{
		Subtle.Render("split: " + s.SplitName),
		"",
		label,
		"",
		Countdown.Render(s.Remaining),
		"",
		Bar(s.ElapsedFrac, s.BarWidth, barStyle),
	}

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
	FocusMins int
	BreakMins int

	// Sent and Failed count the desktop notifications that were actually
	// attempted, so a session ended by skipping them does not claim they failed.
	Sent   int
	Failed int

	Width int
}

func (s Summary) View() string {
	// Stated as the plan rather than as an achievement: skipping a phase early
	// means less time was actually focused than the split calls for.
	plan := TextStyle.Render(fmt.Sprintf("%d min focus  ·  %d min break", s.FocusMins, s.BreakMins))

	body := []string{
		"",
		SuccessStyle.Render("✓  split complete"),
		"",
		TextStyle.Render(s.SplitName),
		plan,
	}

	if notice := s.notice(); notice != "" {
		body = append(body, "", notice)
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

// Help renders the key hints for a screen, wrapped to fit the terminal.
//
// Wrapping matters on a narrow window: a hint line that runs off the edge is
// simply lost, so the keys that do not fit would be invisible.
func Help(width int, entries ...[2]string) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, KeyCap.Render(e[0])+" "+KeyStyle.Render(e[1]))
	}

	line := strings.Join(parts, Subtle.Render("   ·   "))
	if width > 0 {
		line = lipgloss.NewStyle().Width(width).Render(line)
	}

	return line
}
