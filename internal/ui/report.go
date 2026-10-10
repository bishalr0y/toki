package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/bishalr0y/toki/internal/history"
	"github.com/bishalr0y/toki/internal/timer"
)

// The tables are rendered by lipgloss rather than laid out by hand with padded
// strings. A hand-laid table has to be right about the width of every cell and
// re-derived by hand each time a column changes; this one computes it.

// tableHeader is the header row's index, as the table package defines it.
const tableHeader = table.HeaderRow

// tableStyles decides how each cell is painted.
//
// Padding is the important part. The table renderer sizes columns to their
// contents and applies no spacing of its own, so without it the cells run
// together — "2026-10-10long" rather than a date and a split name.
//
// The header is bold and takes the wordmark's colour, and the body inherits the
// terminal's own foreground so it stays readable on a light or dark theme alike —
// the same reason Text is left unset everywhere else in toki.
func tableStyles(row, _ int) lipgloss.Style {
	if row == tableHeader {
		return lipgloss.NewStyle().Bold(true).Foreground(Mauve).Padding(0, 1)
	}
	return TextStyle.Padding(0, 1)
}

// newTable builds a table with toki's borders and the given columns.
//
// Only the outer box and the header rule are drawn. Row rules and column rules
// would be a grid around data that is mostly short strings, and at this size it
// reads as a spreadsheet rather than a summary.
// SectionTitle is the heading row above a printed block.
//
// Italic as well as bold. The headings and the column headers carry the same weight
// otherwise, and they are a line apart — bold blue beside bold mauve reads as two
// headings, when only one of them labels columns. The slant is what separates them
// without having to shout louder.
//
// Colour is Blue rather than the wordmark's mauve, which the column headers
// already use, so the two are told apart by hue as well as by shape. Blue is used
// for key caps on the interactive screens, which never appear alongside this, so
// nothing is competing for it here.
var SectionTitle = lipgloss.NewStyle().Bold(true).Italic(true).Foreground(Blue)

func newTable(headers ...string) *table.Table {
	return table.New().
		Headers(headers...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(Muted)).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		BorderHeader(true).
		BorderColumn(false).
		BorderRow(false).
		StyleFunc(tableStyles)
}

// ReportSection is a heading and the block it labels.
//
// The rule on the heading line is what makes the heading read as a label rather
// than as text that drifted above a table. Left bare, "today" sits on its own and
// the gap between it and the box below reads as something missing; the rule closes
// that gap and ties the two together.
//
// Its length is measured from the block it labels rather than fixed, so the rule
// always ends where the block does. A constant length would either stop short of a
// wide table or run past a narrow one, and that mismatch is the exact thing the
// rule exists to remove.
func ReportSection(heading, body string) string {
	head := SectionTitle.Render(heading)

	// Measured on the first line because that is the block's widest: for a table it
	// is the top border, which is exactly the width the columns settled on.
	first, _, _ := strings.Cut(body, "\n")

	const gap = 2
	rule := lipgloss.Width(first) - lipgloss.Width(heading) - gap

	// Below three the rule is a stub rather than a line, which reads worse than no
	// rule at all — so a heading wider than its block simply gets none.
	if rule < 3 {
		return trimRight(lipgloss.JoinVertical(lipgloss.Left, head, "", body))
	}

	line := head + strings.Repeat(" ", gap) + Subtle.Render(strings.Repeat("─", rule))

	return trimRight(lipgloss.JoinVertical(lipgloss.Left, line, "", body))
}

// trimRight strips the padding lipgloss adds to a block.
//
// JoinVertical pads every line out to the width of the widest one, so the heading
// line and the lines under a table come back with trailing spaces. They are
// invisible on screen but they are in the bytes, and this output is meant to be
// piped, redirected and diffed — where trailing whitespace shows up as a change
// that is not one.
//
// Only spaces are removed, and only from the end, so the escape sequence that
// closes a styled line keeps its trailing reset where it belongs.
func trimRight(block string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}

	return strings.Join(lines, "\n")
}

// empty is what a section says when it has nothing to show.
//
// A sentence rather than an empty box. A box drawn around nothing draws attention
// to the absence, and a bordered table with no rows looks like a rendering fault
// rather than a report of no sessions.
func empty(message string) string {
	return Subtle.Render(message)
}

// TodayTable renders today's totals as a one row table.
//
// A table rather than a sentence because the figures are separate quantities —
// time, rounds, splits — and a table makes them line up and be compared without
// reading each one.
func TodayTable(t history.Totals) string {
	if t.Splits == 0 {
		return empty("nothing recorded yet today")
	}

	return newTable("focused", "rounds", "splits").
		Row(
			timer.FormatTotal(t.Focused),
			strconv.Itoa(t.Rounds),
			strconv.Itoa(t.Splits),
		).
		Render()
}

// HistoryTable renders every record in the window, newest first, grouped by day.
//
// One table rather than one per day: separate boxes would repeat the headers for
// every day and make the whole thing longer than the information it carries. The
// date is a column instead, and it is blank on the rows that continue the day
// above, so each day's records stay visually attached to it.
func HistoryTable(days []history.Day, total history.Totals) string {
	if len(days) == 0 {
		return empty("nothing recorded yet")
	}

	t := newTable("date", "split", "focused", "rounds", "ended")

	for _, day := range days {
		for i, r := range day.Items {
			// Only the first row of a day carries the date, so the records below it
			// read as belonging to that date rather than as rows of their own.
			date := ""
			if i == 0 {
				date = day.Date
			}

			name := r.Name
			if name == "" {
				name = "(unnamed)"
			}

			t.Row(
				date,
				name,
				timer.FormatTotal(r.Focused),
				strconv.Itoa(r.Rounds),
				r.EndedAt.Format("15:04"),
			)
		}
	}

	out := t.Render()

	// The window's grand total, under the table rather than as a summary row
	// inside it: a total row would read as another day's record, and it is not
	// one.
	return out + "\n\n" + Subtle.Render(fmt.Sprintf("%s  ·  %d %s  ·  %d %s",
		timer.FormatTotal(total.Focused),
		total.Rounds, Plural(total.Rounds, "round", "rounds"),
		total.Splits, Plural(total.Splits, "split", "splits")))
}
