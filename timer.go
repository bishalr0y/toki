package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gosuri/uilive"
)

func StartSession(name string, duration int) {
	writer := uilive.New()
	writer.Start()

	for i := range duration {
		fmt.Fprintf(writer, "%s session\n⏳time remaining -> %s\n", name, FormatTimer(duration-i))
		time.Sleep(time.Second)
	}
	fmt.Fprintf(writer, "\n%s session completed!\n", name)
	writer.Stop()
}

func FormatTimer(duration int) string {
	mins := strconv.Itoa(duration / 60)
	seconds := strconv.Itoa(duration % 60)

	if len(mins) == 1 {
		mins = "0" + mins
	}
	if len(seconds) == 1 {
		seconds = "0" + seconds
	}
	return fmt.Sprintf("%s:%s", mins, seconds)
}
