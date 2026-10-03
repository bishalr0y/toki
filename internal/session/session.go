// Package session drives a timer split to completion.
//
// It owns everything the pure timer deliberately does not: rendering, sleeping,
// and desktop notifications. The timing decisions all live in
// internal/timer, which is why this package is thin glue.
//
// This blocking loop is the current UI. Phase 2 replaces it with a Bubble Tea
// event loop, at which point this package becomes unnecessary and its loop
// logic should move into the session model there.
package session

import (
	"fmt"
	"os"
	"time"

	"github.com/bishalr0y/toki/internal/notify"
	"github.com/bishalr0y/toki/internal/timer"
	"github.com/bishalr0y/toki/internal/ui"
	"github.com/gosuri/uilive"
)

// Run plays a split through focus and then break, blocking until both finish.
//
// Progress is derived from an absolute deadline rather than by counting ticks,
// so a slow render or a suspended process cannot make the session run long.
// A failed desktop notification is reported rather than discarded: on Linux and
// Wayland it commonly fails, and silence there looks like the app is broken.
func Run(split timer.Split) error {
	tr := timer.New(split, time.Now())

	writer := uilive.New()
	writer.Start()
	defer writer.Stop()

	for {
		now := time.Now()

		if tr.Expired(now) {
			ui.GreenBold.Fprintf(writer, "\n%s session completed!\n", tr.Phase())

			if err := notify.Notify(tr.Phase().String()); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not send desktop notification: %v\n", err)
			}

			tr.Skip(now)
			if tr.Finished() {
				return nil
			}
			continue
		}

		ui.MauveBold.Fprintf(writer, "%s session\n⏳time remaining -> %s\n",
			tr.Phase(), timer.Format(tr.Remaining(now)))

		// Sleep to the next whole second so the displayed countdown stays in
		// step with the wall clock.
		time.Sleep(timer.NextTick(now))
	}
}
