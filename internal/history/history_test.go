package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// start is a fixed instant so assertions are absolute clock readings.
var start = time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)

func finished(split string, startedAt, endedAt time.Time, focus time.Duration, rounds int) Entry {
	return Entry{
		Split:     split,
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Focus:     focus,
		Rounds:    rounds,
	}
}

// A split that has just finished is the thing worth remembering, so it has to
// survive a round trip through the file.
func TestAppendRecordsACompletedSplit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store := NewStore(path)

	want := finished("Classic", start, start.Add(2*time.Hour), 95*time.Minute, 4)
	if err := store.Append(want); err != nil {
		t.Fatalf("Append() = %v", err)
	}

	got, err := store.Entries()
	if err != nil {
		t.Fatalf("Entries() = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0] != want {
		t.Errorf("entry = %+v, want %+v", got[0], want)
	}
}

// The file is written where the user will look for it, and the directory may not
// exist yet on a first run.
func TestAppendCreatesTheDirectoryItNeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "history.json")
	store := NewStore(path)

	if err := store.Append(finished("Classic", start, start, time.Minute, 1)); err != nil {
		t.Fatalf("Append() = %v; the store should create its own directory", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("history file not written: %v", err)
	}
}

// History is read oldest first, so it reads in the order it happened.
func TestEntriesComeBackOldestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store := NewStore(path)

	for i := range 3 {
		at := start.Add(time.Duration(i) * time.Hour)
		if err := store.Append(finished("Classic", at, at, 95*time.Minute, 4)); err != nil {
			t.Fatalf("Append() = %v", err)
		}
	}

	got, err := store.Entries()
	if err != nil {
		t.Fatalf("Entries() = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3", len(got))
	}
	for i, e := range got {
		want := start.Add(time.Duration(i) * time.Hour)
		if !e.StartedAt.Equal(want) {
			t.Errorf("entry[%d].StartedAt = %v, want %v", i, e.StartedAt, want)
		}
	}
}

// A first run has no history, which is not an error.
func TestAMissingFileIsEmptyHistoryRatherThanAFailure(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "history.json"))

	got, err := store.Entries()
	if err != nil {
		t.Fatalf("Entries() = %v, want a missing file to read as no history", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d entries from a missing file, want 0", len(got))
	}

	totals, err := store.TotalsSince(start)
	if err != nil {
		t.Fatalf("TotalsSince() = %v", err)
	}
	if totals != (Totals{}) {
		t.Errorf("TotalsSince() = %+v, want the zero value", totals)
	}
}

// A file that exists but holds nothing yet is empty history, not corruption.
func TestAnEmptyFileIsEmptyHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewStore(path).Entries()
	if err != nil {
		t.Fatalf("Entries() = %v, want an empty file to read as no history", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d entries from an empty file, want 0", len(got))
	}
}

// Corrupt history is the user's to fix, and it has to say so. Swallowing it
// would make the totals quietly wrong, and because Append has to read the file
// before it can add to it, pretending everything is fine would also stop future
// splits being recorded at all.
func TestCorruptHistoryIsReportedRatherThanIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(path)

	if _, err := store.Entries(); err == nil {
		t.Error("Entries() = nil error for a corrupt file, want a failure")
	}
	if err := store.Append(finished("Classic", start, start, time.Minute, 1)); err == nil {
		t.Error("Append() = nil error for a corrupt file, want a failure")
	}
}

// TotalsSince answers "how much have I done today", which is what the summary
// shows alongside the session's own total.
func TestTotalsSinceCountsOnlyWhatCameAfter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store := NewStore(path)

	midnight := start
	today := start.Add(12 * time.Hour)
	yesterday := start.Add(-12 * time.Hour)

	for _, e := range []Entry{
		finished("Classic", yesterday, yesterday.Add(time.Hour), 95*time.Minute, 4),
		finished("Classic", today, today.Add(2*time.Hour), 95*time.Minute, 4),
		finished("Double", today, today.Add(3*time.Hour), 50*time.Minute, 1),
	} {
		if err := store.Append(e); err != nil {
			t.Fatalf("Append() = %v", err)
		}
	}

	got, err := store.TotalsSince(midnight)
	if err != nil {
		t.Fatalf("TotalsSince() = %v", err)
	}

	want := Totals{Splits: 2, Focus: 145 * time.Minute}
	if got != want {
		t.Errorf("TotalsSince(midnight) = %+v, want %+v; yesterday must not count", got, want)
	}
}

// A split that ended exactly at midnight belongs to today, not to yesterday:
// "since" has to include the boundary or the first split of the day is lost.
func TestTotalsSinceIncludesTheBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store := NewStore(path)

	at := start
	if err := store.Append(finished("Classic", at.Add(-time.Hour), at, 25*time.Minute, 1)); err != nil {
		t.Fatalf("Append() = %v", err)
	}

	got, err := store.TotalsSince(at)
	if err != nil {
		t.Fatalf("TotalsSince() = %v", err)
	}
	if want := (Totals{Splits: 1, Focus: 25 * time.Minute}); got != want {
		t.Errorf("TotalsSince(at) = %+v, want %+v", got, want)
	}
}

// The file is meant to be readable and editable, so a duration is stored as a
// number of seconds rather than a nanosecond count.
func TestTheFileIsReadableRatherThanNanoseconds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	store := NewStore(path)

	if err := store.Append(finished("Classic", start, start.Add(time.Hour), 95*time.Minute, 4)); err != nil {
		t.Fatalf("Append() = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), "5700000000000") {
		t.Errorf("history holds a nanosecond count:\n%s", data)
	}
	if !strings.Contains(string(data), "5700") {
		t.Errorf("history should hold 5700 seconds (95m), got:\n%s", data)
	}
}

// A crash part way through a write must not leave a half-written file behind,
// because that file is then unreadable and every later split is lost too.
func TestAppendLeavesNoHalfWrittenFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	store := NewStore(path)

	if err := store.Append(finished("Classic", start, start, time.Minute, 1)); err != nil {
		t.Fatalf("Append() = %v", err)
	}

	// A rename-based write leaves nothing else in the directory; a write-in-place
	// or a leftover temporary would show up here.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "history.json" {
			t.Errorf("Append() left %q behind", e.Name())
		}
	}
}

// "Today" has to mean the user's day, not UTC's. Truncating in UTC would put the
// boundary in the wrong place for most of the world: a session finished at 00:30
// in UTC+5 belongs to the new day, not the one before.
func TestMidnightUsesTheLocalDayNotUTCs(t *testing.T) {
	for _, c := range []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "ahead of UTC",
			now:  time.Date(2026, 10, 3, 0, 30, 0, 0, time.FixedZone("east", 5*3600)),
			want: time.Date(2026, 10, 3, 0, 0, 0, 0, time.FixedZone("east", 5*3600)),
		},
		{
			name: "behind UTC, late in the evening",
			now:  time.Date(2026, 10, 3, 23, 30, 0, 0, time.FixedZone("west", -7*3600)),
			want: time.Date(2026, 10, 3, 0, 0, 0, 0, time.FixedZone("west", -7*3600)),
		},
		{
			name: "just after midnight",
			now:  time.Date(2026, 10, 3, 0, 0, 1, 0, time.FixedZone("east", 5*3600)),
			want: time.Date(2026, 10, 3, 0, 0, 0, 0, time.FixedZone("east", 5*3600)),
		},
		{
			name: "last instant of the day",
			now:  time.Date(2026, 10, 3, 23, 59, 59, 0, time.FixedZone("east", 5*3600)),
			want: time.Date(2026, 10, 3, 0, 0, 0, 0, time.FixedZone("east", 5*3600)),
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Midnight(c.now)
			if !got.Equal(c.want) {
				t.Errorf("Midnight(%v) = %v, want %v", c.now, got, c.want)
			}
			if got.After(c.now) {
				t.Errorf("Midnight(%v) = %v, which is after the day has started", c.now, got)
			}
		})
	}
}

// The boundary has to keep the offset it was given. Rebuilding midnight in a
// different zone would silently shift the day for anyone west of UTC.
func TestMidnightKeepsTheGivenOffset(t *testing.T) {
	now := time.Date(2026, 10, 3, 14, 0, 0, 0, time.FixedZone("east", 5*3600))

	got := Midnight(now)
	if _, offset := got.Zone(); offset != 5*3600 {
		t.Errorf("Midnight kept offset %d, want %d", offset, 5*3600)
	}
}
