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

// A session total routinely passes an hour, which Format cannot express without
// spilling out of MM:SS into something like "145:00".
func TestFormatTotalPromotesToHours(t *testing.T) {
	steps := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Minute, "45m"},
		{59*time.Minute + 59*time.Second, "59m"},
		{time.Hour, "1h"},
		{time.Hour + 30*time.Minute, "1h 30m"},
		{2*time.Hour + 5*time.Minute, "2h 05m"},
		{145 * time.Minute, "2h 25m"},
		{-5 * time.Minute, "0s"},
	}

	for _, s := range steps {
		if got := FormatTotal(s.in); got != s.want {
			t.Errorf("FormatTotal(%v) = %q, want %q", s.in, got, s.want)
		}
	}
}

// Seconds are noise at the scale of a total, and a total is a summary rather than
// a countdown, so they are dropped rather than shown.
func TestFormatTotalDropsSeconds(t *testing.T) {
	if got, want := FormatTotal(25*time.Minute+40*time.Second), "25m"; got != want {
		t.Errorf("FormatTotal = %q, want %q", got, want)
	}
}

// A session abandoned moments in has nothing to round to, and "0m" for that
// reads like a failure rather than a fact.
func TestFormatTotalShowsSecondsBelowAMinute(t *testing.T) {
	for _, c := range []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
	} {
		if got := FormatTotal(c.in); got != c.want {
			t.Errorf("FormatTotal(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
