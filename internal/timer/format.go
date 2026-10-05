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

// FormatTotal renders an accumulated duration as hours and minutes.
//
// A session total routinely passes an hour, which Format cannot express without
// spilling out of its MM:SS shape into something like "145:00". Seconds are
// dropped because they are noise at that scale and the number is a summary
// rather than a countdown.
func FormatTotal(d time.Duration) string {
	mins := max(int(d/time.Minute), 0)

	hours, mins := mins/60, mins%60
	switch {
	case hours == 0 && mins == 0:
		// Nothing to round to, and "0m" for a split abandoned moments in reads
		// like a failure rather than a fact.
		return fmt.Sprintf("%ds", max(int(d/time.Second), 0))
	case hours == 0:
		return fmt.Sprintf("%dm", mins)
	case mins == 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dh %02dm", hours, mins)
	}
}
