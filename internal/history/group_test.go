package history

import (
	"testing"
	"time"
)

// the history is looking for.
func TestDaysAreGroupedNewestFirst(t *testing.T) {
	records := []Record{
		{Name: "oldest", EndedAt: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)},
		{Name: "newest", EndedAt: time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)},
		{Name: "middle", EndedAt: time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)},
	}

	days := GroupByDay(records)
	if len(days) != 3 {
		t.Fatalf("got %d days, want 3", len(days))
	}

	want := []string{"2026-10-08", "2026-10-07", "2026-10-06"}
	for i, date := range want {
		if days[i].Date != date {
			t.Errorf("days[%d].Date = %q, want %q", i, days[i].Date, date)
		}
	}
}

// then be wrong.
func TestRecordsWithinADayAreNewestFirst(t *testing.T) {
	day := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	records := []Record{
		{Name: "morning", EndedAt: day.Add(9 * time.Hour)},
		{Name: "evening", EndedAt: day.Add(21 * time.Hour)},
		{Name: "lunch", EndedAt: day.Add(13 * time.Hour)},
	}

	days := GroupByDay(records)
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}

	want := []string{"evening", "lunch", "morning"}
	for i, name := range want {
		if days[0].Items[i].Name != name {
			t.Errorf("items[%d] = %q, want %q", i, days[0].Items[i].Name, name)
		}
	}
}

func TestADayTotalsItsOwnRecords(t *testing.T) {
	day := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	records := []Record{
		{Name: "a", Focused: 25 * time.Minute, Rounds: 2, EndedAt: day.Add(9 * time.Hour)},
		{Name: "b", Focused: 50 * time.Minute, Rounds: 4, EndedAt: day.Add(21 * time.Hour)},
	}

	days := GroupByDay(records)
	if got := days[0].Total.Focused; got != 75*time.Minute {
		t.Errorf("Focused = %v, want %v", got, 75*time.Minute)
	}
	if got := days[0].Total.Rounds; got != 6 {
		t.Errorf("Rounds = %d, want 6", got)
	}
	if got := days[0].Total.Splits; got != 2 {
		t.Errorf("Splits = %d, want 2", got)
	}
}
