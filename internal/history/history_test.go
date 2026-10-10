package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// home points HOME at a fresh directory, so no test can see or write the real
// one's history.
func home(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
	return dir
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestAnEmptyHistoryIsNotAFailure(t *testing.T) {
	home(t)

	records, err := Since(at("2026-01-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("Since() returned error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Since() = %d records, want 0", len(records))
	}
}

func TestARecordSurvivesBeingReadBack(t *testing.T) {
	home(t)

	want := Record{Name: "focus", Focused: 25 * time.Minute, Rounds: 4, EndedAt: at("2026-10-08T14:32:00Z")}
	if err := Append(want); err != nil {
		t.Fatalf("Append() returned error: %v", err)
	}

	records, err := Since(at("2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("Since() returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("Since() = %d records, want 1", len(records))
	}

	got := records[0]
	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if got.Focused != want.Focused {
		t.Errorf("Focused = %v, want %v", got.Focused, want.Focused)
	}
	if got.Rounds != want.Rounds {
		t.Errorf("Rounds = %d, want %d", got.Rounds, want.Rounds)
	}
	if !got.EndedAt.Equal(want.EndedAt) {
		t.Errorf("EndedAt = %v, want %v", got.EndedAt, want.EndedAt)
	}
}

// The file is line-delimited, which is the property that lets one bad line cost
// one record instead of the whole file. A record written as several lines would
// break that, so it is pinned rather than left to the encoding to stay put.
func TestEachRecordIsExactlyOneLine(t *testing.T) {
	home(t)

	for _, name := range []string{"focus", "short", "a name with spaces", "unicode ✓"} {
		if err := Append(Record{Name: name, Focused: time.Minute, Rounds: 1, EndedAt: time.Now()}); err != nil {
			t.Fatalf("Append(%q) returned error: %v", name, err)
		}
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}
	for _, line := range lines {
		if strings.Contains(line, "\n") {
			t.Errorf("a record spans more than one line: %q", line)
		}
	}
}

// A machine that loses power mid-write leaves a partial final line. One
// unreadable record must not cost the user every record before it.
func TestATornLastLineDoesNotCostTheEarlierOnes(t *testing.T) {
	home(t)

	good := Record{Name: "focus", Focused: 25 * time.Minute, Rounds: 4, EndedAt: at("2026-10-08T14:32:00Z")}
	if err := Append(good); err != nil {
		t.Fatal(err)
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"name":"half-writ`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	records, err := Since(at("2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("Since() returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("Since() = %d records, want 1", len(records))
	}
	if records[0].Name != good.Name {
		t.Errorf("Name = %q, want %q", records[0].Name, good.Name)
	}
}

func TestSinceLeavesOutEarlierRecords(t *testing.T) {
	home(t)

	for _, r := range []Record{
		{Name: "old", EndedAt: at("2026-09-01T10:00:00Z")},
		{Name: "recent", EndedAt: at("2026-10-08T10:00:00Z")},
	} {
		if err := Append(r); err != nil {
			t.Fatal(err)
		}
	}

	records, err := Since(at("2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("Since() = %d records, want 1", len(records))
	}
	if records[0].Name != "recent" {
		t.Errorf("Name = %q, want %q", records[0].Name, "recent")
	}
}

// Records come back oldest first because the file is already in that order, and
// sorting on read would mean the caller had to know it had been undone.
func TestRecordsComeBackOldestFirst(t *testing.T) {
	home(t)

	for _, r := range []Record{
		{Name: "first", EndedAt: at("2026-10-08T09:00:00Z")},
		{Name: "second", EndedAt: at("2026-10-08T10:00:00Z")},
		{Name: "third", EndedAt: at("2026-10-08T11:00:00Z")},
	} {
		if err := Append(r); err != nil {
			t.Fatal(err)
		}
	}

	records, err := Since(at("2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"first", "second", "third"}
	if len(records) != len(want) {
		t.Fatalf("Since() = %d records, want %d", len(records), len(want))
	}
	for i, name := range want {
		if records[i].Name != name {
			t.Errorf("records[%d].Name = %q, want %q", i, records[i].Name, name)
		}
	}
}

// Today's total resets at local midnight, not twenty-four hours ago. A total that
// reset at a moving moment would be a number nobody could predict.
func TestTodayResetsAtLocalMidnight(t *testing.T) {
	home(t)

	now := time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC)

	for _, r := range []Record{
		{Name: "yesterday", Focused: 50 * time.Minute, Rounds: 4, EndedAt: now.AddDate(0, 0, -1)},
		{Name: "today", Focused: 25 * time.Minute, Rounds: 4, EndedAt: now.Add(-30 * time.Minute)},
	} {
		if err := Append(r); err != nil {
			t.Fatal(err)
		}
	}

	totals, err := Today(now)
	if err != nil {
		t.Fatal(err)
	}

	if totals.Focused != 25*time.Minute {
		t.Errorf("Focused = %v, want %v", totals.Focused, 25*time.Minute)
	}
	if totals.Rounds != 4 {
		t.Errorf("Rounds = %d, want 4", totals.Rounds)
	}
	if totals.Splits != 1 {
		t.Errorf("Splits = %d, want 1", totals.Splits)
	}
}

// The boundary is the viewer's own midnight, not UTC's. Someone in UTC+5:30
// should not have their day start at the wrong hour.
func TestTodayUsesTheLocalMidnightNotUTC(t *testing.T) {
	home(t)

	loc := time.FixedZone("UTC+5:30", int(5*time.Hour/time.Second)+int(30*time.Minute/time.Second))

	// 04:00 local on the 8th, which is 22:30 UTC on the 7th.
	now := time.Date(2026, 10, 8, 4, 0, 0, 0, loc)

	if err := Append(Record{Name: "today", Focused: time.Hour, Rounds: 2, EndedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}

	totals, err := Today(now)
	if err != nil {
		t.Fatal(err)
	}
	if totals.Focused != time.Hour {
		t.Errorf("Focused = %v, want %v", totals.Focused, time.Hour)
	}
}

// The window is three calendar months, not ninety days, so the two drift apart
// over a year and the boundary has to be pinned deliberately.
func TestTheWindowIsThreeCalendarMonths(t *testing.T) {
	home(t)

	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

	for _, r := range []Record{
		{Name: "too-old", Focused: time.Hour, EndedAt: now.AddDate(0, -4, 0)},
		{Name: "inside", Focused: 25 * time.Minute, EndedAt: now.AddDate(0, -1, 0)},
	} {
		if err := Append(r); err != nil {
			t.Fatal(err)
		}
	}

	totals, err := Window(now)
	if err != nil {
		t.Fatal(err)
	}

	if totals.Splits != 1 {
		t.Fatalf("Splits = %d, want 1", totals.Splits)
	}
	if totals.Focused != 25*time.Minute {
		t.Errorf("Focused = %v, want %v", totals.Focused, 25*time.Minute)
	}
}

func TestTheHistoryFileSitsBesideTheConfig(t *testing.T) {
	dir := home(t)

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(dir, ".config/toki", File)
	if path != want {
		t.Errorf("Path() = %q, want %q", path, want)
	}
}

// The whole reason for a text file: removing it leaves nothing behind.
func TestRemovingTheFileErasesEverything(t *testing.T) {
	home(t)

	if err := Append(Record{Name: "focus", Focused: time.Minute, Rounds: 1, EndedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	// Everything else in the directory is left alone, so deleting the history is
	// not the same as deleting the config.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == File {
			t.Errorf("%s is still present after removing it", File)
		}
	}

	totals, err := Today(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if totals.Splits != 0 {
		t.Errorf("Splits = %d, want 0 after erasing the file", totals.Splits)
	}
}

// The file is meant to be readable by whoever finds it, so focused_s has to mean
// seconds. Go marshals a time.Duration as nanoseconds, which would make the field
// name a lie.
func TestFocusedIsWrittenInSeconds(t *testing.T) {
	home(t)

	r := Record{Name: "focus", Focused: 25*time.Minute + 30*time.Second, Rounds: 4, EndedAt: at("2026-10-08T14:32:00Z")}
	if err := Append(r); err != nil {
		t.Fatal(err)
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), `"focused_s":1530`) {
		t.Errorf("record does not carry seconds:\n%s", data)
	}
}

// Whole seconds is the resolution, so a fraction is truncated rather than
// rounded. In practice a split focuses for whole minutes and this never fires;
// it is pinned so that the loss is a decision rather than a surprise.
func TestSubSecondPrecisionIsTruncatedNotRounded(t *testing.T) {
	home(t)

	r := Record{Name: "focus", Focused: 1500 * time.Millisecond, Rounds: 1, EndedAt: at("2026-10-08T14:32:00Z")}
	if err := Append(r); err != nil {
		t.Fatal(err)
	}

	records, err := Since(at("2026-10-01T00:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if got := records[0].Focused; got != time.Second {
		t.Errorf("Focused = %v, want %v", got, time.Second)
	}
}
