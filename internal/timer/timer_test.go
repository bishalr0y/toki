package timer

import "testing"

func TestFormatTimer(t *testing.T) {
	tests := []struct {
		duration int
		want     string
	}{
		{0, "00:00"},
		{1, "00:01"},
		{59, "00:59"},
		{60, "01:00"},
		{61, "01:01"},
		{150, "02:30"},
		{3600, "60:00"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatTimer(tt.duration)
			if got != tt.want {
				t.Errorf("FormatTimer(%d) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}
