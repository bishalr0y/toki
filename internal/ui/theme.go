// Package ui renders every screen toki draws.
//
// Styling lives here rather than being scattered through the model so that the
// Bubble Tea layer deals only in state and events.
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Accent colours, carried over unchanged from the pre-Bubble Tea ui/colors.go
// so the existing look survives the rewrite.
var (
	Blue  = lipgloss.Color("#8AADF4")
	Red   = lipgloss.Color("#ED8796")
	Green = lipgloss.Color("#A6DA95")
	Mauve = lipgloss.Color("#CA9AE6")
	Peach = lipgloss.Color("#EF9F76")

	// Neutrals, chosen to sit quietly behind the accents above.
	Text  = lipgloss.Color("#4C4F69")
	Muted = lipgloss.Color("#8C8FA1")
	Base  = lipgloss.Color("#EFF1F5")
)

// Styles shared across screens.
var (
	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(Mauve)
	Subtle     = lipgloss.NewStyle().Foreground(Muted)
	TextStyle  = lipgloss.NewStyle().Foreground(Text)

	// Picker's selected row.
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Base).
			Background(Peach).
			Padding(0, 2)

	// Picker's unselected rows, indented to line up with the selected one.
	RowStyle = lipgloss.NewStyle().Foreground(Text).Padding(0, 2)

	// The large phase label above the countdown.
	FocusLabel  = lipgloss.NewStyle().Bold(true).Foreground(Peach)
	BreakLabel  = lipgloss.NewStyle().Bold(true).Foreground(Green)
	PausedLabel = lipgloss.NewStyle().Bold(true).Foreground(Red)

	// The countdown digits themselves.
	Countdown = lipgloss.NewStyle().Bold(true).Foreground(Mauve)

	// Focus and break get different bar colours so the phase is readable at a
	// glance without reading the label.
	FocusBar = lipgloss.NewStyle().Foreground(Peach)
	BreakBar = lipgloss.NewStyle().Foreground(Green)

	ErrorStyle = lipgloss.NewStyle().Foreground(Red)

	// The summary screen's good news.
	SuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(Green)

	// Key hints along the bottom of a screen.
	KeyStyle = lipgloss.NewStyle().Foreground(Text)
	KeyCap   = lipgloss.NewStyle().Bold(true).Foreground(Blue)
)

// Banner is the toki wordmark.
const Banner = `  __          __   .__
_/  |_  ____ |  | _|__|
\   __\/  _ \|  |/ /  |
 |  | (  <_> )    <|  |
 |__|  \____/|__|_ \__|
                   \/`

// Title renders the wordmark.
func Title() string { return TitleStyle.Render(Banner) }

// LabelFor returns the styled label for a phase.
//
// Pausing is reported on its own line rather than by replacing this, so the
// phase is still readable while the countdown is held.
func LabelFor(phase string) string {
	if phase == "BREAK" {
		return BreakLabel.Render(phase)
	}
	return FocusLabel.Render(phase)
}

// Bar renders a progress bar of the given width, filled to fraction in [0,1].
//
// Rounding is done on the filled width so the bar never renders more than
// width cells, which would otherwise wrap and corrupt the layout.
func Bar(fraction float64, width int, style lipgloss.Style) string {
	if width < 1 {
		width = 1
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}

	filled := min(int(fraction*float64(width)+0.5), width)

	return style.Render(strings.Repeat("━", filled) + strings.Repeat("─", width-filled))
}
