package ui

import (
	"fmt"

	"github.com/bishalr0y/toki/internal/history"
	"github.com/bishalr0y/toki/internal/timer"
)

// TodayLine is the day's running total, shown on the split picker and under the
// countdown while a session runs.
//
// It is one line rather than a block because both places that show it are
// interactive screens doing something else: the picker is a list to choose from
// and the session screen is a timer to watch.
//
// Blank until there is a figure to show. That covers both an empty day and the
// first instant of the first round, where the running total is still zero —
// "today 0s" there reads as a counter that is broken rather than a day that has
// just started.
func TodayLine(t history.Totals) string {
	if t.Focused <= 0 && t.Rounds == 0 {
		return ""
	}

	plural := "rounds"
	if t.Rounds == 1 {
		plural = "round"
	}

	return fmt.Sprintf("today %s  ·  %d %s", timer.FormatTotal(t.Focused), t.Rounds, plural)
}
