package ui

import (
	"os"

	"github.com/bishalr0y/toki/internal/config"
	"github.com/jedib0t/go-pretty/v6/table"
)

func PrintAvailableSplits(cfg config.Config) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Split Name", "Focus(mins)", "Break(mins)"})
	for i, timer := range cfg.Timers {
		t.AppendRow([]any{i + 1, timer.Name, timer.Focus, timer.Break})
	}
	t.Render()
}
