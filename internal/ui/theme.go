// Package ui renders every screen toki draws.
//
// Styling lives here rather than being scattered through the model so that the
// Bubble Tea layer deals only in state and events.
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/bishalr0y/toki/internal/timer"
)

// The sixteen colours every terminal provides, and nothing else.
//
// Nothing here is a fixed RGB value on purpose. A hex code has to guess what your
// terminal's background is, and it guesses wrong the moment that background is not
// the one it was designed against — unreadable text on a light theme, or a bar that
// disappears into a dark one. An ANSI colour is a request for "whatever you use for
// bright red", which every terminal already has an answer for and every theme has
// tuned. So toki looks the same on a light background, a dark one and a solarized
// one, because in every case the terminal decides.
//
// The values are the bright half of the palette where one is wanted, since these
// are accents against a background we do not know.
const (
	ansiBlack   = "0"
	ansiRed     = "9"
	ansiGreen   = "10"
	ansiYellow  = "11"
	ansiBlue    = "12"
	ansiMagenta = "13"
	// Bright black is the colour terminals conventionally use for dimmed text, so
	// it is the one to reach for when something should be quieter than the rest.
	ansiDim = "8"
)

var (
	Blue  = lipgloss.Color(ansiBlue)
	Red   = lipgloss.Color(ansiRed)
	Green = lipgloss.Color(ansiGreen)
	Mauve = lipgloss.Color(ansiMagenta)
	Peach = lipgloss.Color(ansiYellow)

	// Text is deliberately left unset, which is what makes the body of the interface
	// inherit the terminal's own foreground. That is the one colour guaranteed to
	// contrast with its own background, whatever the theme, and asking for anything
	// specific would only throw that guarantee away.
	Text = lipgloss.Color("")

	// Muted is the terminals' dimmed colour, for the lines that carry less weight
	// than the text around them.
	Muted = lipgloss.Color(ansiDim)

	// Base is painted on top of Peach for the highlighted row. Black is the only
	// safe choice there: themes put light colours in the bright half of the
	// palette, so black on a bright bar reads on every one of them.
	Base = lipgloss.Color(ansiBlack)
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
//
// The phase arrives as a timer.Phase rather than a string on purpose. Matching on
// the text meant each screen had to remember the full list of rest phases, and a
// long break came out in the focus colour while its bar came out in the break one.
func LabelFor(phase timer.Phase) string {
	if isRest(phase) {
		return BreakLabel.Render(phase.String())
	}
	return FocusLabel.Render(phase.String())
}

// isRest says whether a phase is a rest rather than work. Every screen that needs to
// know asks here, so the label and the bar cannot disagree about a phase.
func isRest(phase timer.Phase) bool {
	return phase == timer.PhaseBreak || phase == timer.PhaseLongBreak
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
