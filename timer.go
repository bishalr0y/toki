package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gosuri/uilive"
	"github.com/jedib0t/go-pretty/v6/table"
)

func StartSession(name string, duration int) {
	writer := uilive.New()
	writer.Start()

	PrintBanner()
	for i := range duration {
		MauveBold.Fprintf(writer, "%s session\n⏳time remaining -> %s\n", name, FormatTimer(duration-i))
		time.Sleep(time.Second)
	}
	GreenBold.Fprintf(writer, "\n%s session completed!\n", name)
	writer.Stop()
}

func FormatTimer(duration int) string {
	mins := strconv.Itoa(duration / 60)
	seconds := strconv.Itoa(duration % 60)

	// prepend 0
	if len(mins) == 1 {
		mins = "0" + mins
	}
	if len(seconds) == 1 {
		seconds = "0" + seconds
	}
	return fmt.Sprintf("%s:%s", mins, seconds)
}

func PrintAvailableSplits(config Config) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Split Name", "Focus(mins)", "Break(mins)"})
	for i, timer := range config.Timers {
		t.AppendRow([]any{i + 1, timer.Name, timer.Focus, timer.Break})
	}
	t.Render()
}
