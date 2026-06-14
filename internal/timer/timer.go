package timer

import (
	"fmt"
	"time"

	"github.com/bishalr0y/toki/internal/notify"
	"github.com/bishalr0y/toki/internal/ui"
	"github.com/gosuri/uilive"
)

func StartSession(name string, duration int) {
	writer := uilive.New()
	writer.Start()

	for i := range duration {
		ui.MauveBold.Fprintf(writer, "%s session\n⏳time remaining -> %s\n", name, FormatTimer(duration-i))
		time.Sleep(time.Second)
	}
	ui.GreenBold.Fprintf(writer, "\n%s session completed!\n", name)
	_ = notify.Notify(name)
	writer.Stop()
}

func FormatTimer(duration int) string {
	mins := duration / 60
	secs := duration % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}
