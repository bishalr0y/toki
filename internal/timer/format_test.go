package timer

import (
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"zero", 0, "00:00"},
		{"one second", time.Second, "00:01"},
		{"last second of a minute", 59 * time.Second, "00:59"},
		{"exactly a minute", time.Minute, "01:00"},
		{"minute and a second", time.Minute + time.Second, "01:01"},
		{"two and a half minutes", 150 * time.Second, "02:30"},
		{"one hour stays in MM:SS", time.Hour, "60:00"},
		{"fractional seconds truncate down", 1599 * time.Millisecond, "00:01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.in); got != tt.want {
				t.Errorf("Format(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// A phase that has overrun its deadline must never render as a negative clock.
func TestFormatClampsNegativeDurationsToZero(t *testing.T) {
	if got := Format(-5 * time.Second); got != "00:00" {
		t.Errorf("Format(-5s) = %q, want %q", got, "00:00")
	}
}
