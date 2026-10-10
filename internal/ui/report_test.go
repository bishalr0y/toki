package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/bishalr0y/toki/internal/history"
)

// day builds a group for one date. The date is fixed because every test here is
// about what happens within a day, and a parameter would only invite a test to
// vary it and mean something else by the failure.
func day(records ...history.Record) history.Day {
	return history.Day{Date: "2026-10-08", Total: history.Totals{}, Items: records}
}

// The columns have to be separated. The table renderer applies no padding of its
// own, so without it a date and the split name beside it run together and the
// table becomes one long word.
func TestTableCellsAreSeparated(t *testing.T) {
	days := []history.Day{day(
		history.Record{
			Name:    "long",
			Focused: 25 * time.Minute,
			Rounds:  4,
			EndedAt: time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC),
		},
	)}

	out := strip(HistoryTable(days, history.Totals{Focused: 25 * time.Minute}))
	if strings.Contains(out, "2026-10-08long") {
		t.Errorf("the date and split name run together:\n%s", out)
	}
	if !strings.Contains(out, "long") {
		t.Errorf("the split name is missing:\n%s", out)
	}
}

// Only the first row of a day carries the date, so the records under it read as
// belonging to that date rather than as rows in their own right.
func TestOnlyTheFirstRowOfADayCarriesTheDate(t *testing.T) {
	days := []history.Day{day(
		history.Record{Name: "long", Rounds: 4, EndedAt: time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC)},
		history.Record{Name: "short", Rounds: 4, EndedAt: time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)},
	)}

	out := strip(HistoryTable(days, history.Totals{}))
	if got := strings.Count(out, "2026-10-08"); got != 1 {
		t.Errorf("the date appears %d times, want once for the day:\n%s", got, out)
	}
}

func TestAnEmptyTodayTableSaysSo(t *testing.T) {
	out := strip(TodayTable(history.Totals{}))
	if !strings.Contains(out, "nothing recorded yet") {
		t.Errorf("an empty day does not say so:\n%s", out)
	}
}

// A record written before names existed must not leave a hole in the column.
func TestAnUnnamedSplitStillPrints(t *testing.T) {
	days := []history.Day{day(
		history.Record{Focused: time.Minute, Rounds: 1, EndedAt: time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)},
	)}

	if out := strip(HistoryTable(days, history.Totals{})); !strings.Contains(out, "(unnamed)") {
		t.Errorf("a record with no name renders as nothing:\n%s", out)
	}
}

// strip removes styling so the tests can assert on the text rather than on the
// escape codes around it.
func strip(s string) string {
	var b strings.Builder
	inEscape := false

	for _, r := range s {
		switch {
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		case r == 0x1b:
			inEscape = true
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// The heading sits above the box, not inside it. A title row inside the border
// has to be composed by hand, and it reads as a caption anyway — the box is
// already the container, and putting a label in it says something different from
// naming what follows.
func TestTheHeadingIsAboveTheBoxNotInsideIt(t *testing.T) {
	out := strip(tokiReport())

	title := lineWith(out, "today")
	if title == "" {
		t.Fatalf("the heading is missing:\n%s", out)
	}

	box := lineWith(out, "╭")
	if box == "" {
		t.Fatalf("there is no box:\n%s", out)
	}

	if titleAfter(box, title) {
		t.Errorf("the heading is inside the box:\n%s", out)
	}
}

// lineWith returns the first line containing s, or an empty string.
func lineWith(out, s string) string {
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, s) {
			return line
		}
	}

	return ""
}

// titleAfter reports whether title occurs after marker on the same line, which is
// how a heading inside a border looks.
func titleAfter(line, title string) bool {
	box := strings.Index(line, "╭")
	if box < 0 {
		return false
	}

	at := strings.Index(line, title)
	return at > box
}

// The section title and the column headers are different kinds of thing — one
// names the report, the other names the columns — and they sit a line apart. If
// they share a colour they stop being distinguishable, and bold is not enough to
// tell two identically coloured lines apart.
//
// Comparing the escape sequences is the only way to catch this: a test that
// stripped the styling would see two identical strings and pass.
func TestTheSectionTitleIsNotTheSameColourAsTheColumnHeaders(t *testing.T) {
	out := tokiReport()

	title := colourBefore(lineWith(out, "today"), "today")
	header := colourBefore(lineWith(out, "focused"), "focused")

	if title == "" || header == "" {
		t.Fatalf("expected both rows to be styled:\n%s", out)
	}
	if title == header {
		t.Errorf("the section title and the column headers are both %s", title)
	}
}

// colourBefore returns the SGR parameters applied to a word on a line.
//
// It looks backwards from the word rather than forwards from the start of the
// line, because a row inside the box carries the border's dimming first: taking
// the first sequence on the line would return the border colour for every row and
// compare the border against itself, which passes whatever the headings are set to.
func colourBefore(line, word string) string {
	before, _, ok := strings.Cut(line, word)
	if !ok {
		return ""
	}

	esc := strings.LastIndex(before, "\x1b")
	if esc < 0 {
		return ""
	}

	end := strings.Index(line[esc:], "m")
	if end < 0 {
		return ""
	}

	return line[esc+2 : esc+end]
}

// tokiReport is one report with both sections, as the command line prints it.
func tokiReport() string {
	return ReportSection("today",
		TodayTable(history.Totals{Focused: time.Hour, Rounds: 2, Splits: 1}))
}

// The rule is what makes the heading read as a label rather than as text that
// drifted above the table. Without it there is nothing closing the gap between
// the heading and the box.
func TestTheHeadingCarriesARule(t *testing.T) {
	out := strip(tokiReport())

	if !strings.Contains(lineWith(out, "today"), "─") {
		t.Errorf("the heading has no rule beside it:\n%s", out)
	}
}

// The rule is measured from the block it labels, so it always ends where that
// block does. A fixed length would stop short of a wide table or overrun a narrow
// one, and the mismatch is what the rule is meant to remove — so the two widths
// have to agree.
func TestTheRuleEndsWhereTheBlockDoes(t *testing.T) {
	wide := strip(ReportSection("last 3 months",
		HistoryTable([]history.Day{day(
			history.Record{Name: "long", Rounds: 4, EndedAt: time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC)},
		)}, history.Totals{})))

	head := lineWith(wide, "last 3 months")
	box := lineWith(wide, "╭")

	if lipgloss.Width(head) != lipgloss.Width(box) {
		t.Errorf("the rule is %d wide, the block %d:\n%s",
			lipgloss.Width(head), lipgloss.Width(box), wide)
	}
}

// A heading wider than the block it labels would leave no room for a rule, and a
// two character stub reads worse than none at all.
func TestAHeadingWiderThanItsBlockGetsNoRule(t *testing.T) {
	out := strip(ReportSection("a very long heading indeed", "short"))

	if strings.Contains(lineWith(out, "a very long"), "─") {
		t.Errorf("a stubby rule was drawn beside a long heading:\n%s", out)
	}
}

// lipgloss pads a block out to its widest line. Those spaces are invisible on
// screen but they are in the bytes, and this output is piped and diffed.
func TestNoLineCarriesTrailingWhitespace(t *testing.T) {
	out := tokiReport()

	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("a line ends in whitespace: %q", line)
		}
	}
}

// The headings are italic. Bold blue beside bold mauve reads as two headings a
// line apart, and only one of them labels columns — so the slant is what tells
// them apart without either one having to shout louder.
//
// Asserting on the SGR parameters is the only way to see this: the rendered text
// is identical either way.
func TestTheHeadingIsItalic(t *testing.T) {
	out := tokiReport()

	heading := colourBefore(lineWith(out, "today"), "today")
	if !strings.Contains(heading, "3") {
		t.Errorf("the heading is not italic: SGR %q", heading)
	}
}
