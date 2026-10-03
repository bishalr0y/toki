package timer

import (
	"fmt"
	"time"
)

// Format renders a duration as MM:SS.
//
// Durations of an hour or more are intentionally not promoted to HH:MM:SS:
// the longest supported split is 50 minutes, so promoting at one hour would
// only change output no configured timer can produce.
func Format(d time.Duration) string {
	secs := max(int(d/time.Second), 0)
	return fmt.Sprintf("%02d:%02d", secs/60, secs%60)
}
