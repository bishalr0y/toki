// Package history records finished splits and reads them back.
//
// Records are stored one JSON object per line in a plain text file rather than in
// a database, for two reasons. It adds nothing to the binary, and deleting the
// file is a complete erase — a database directory leaves manifests and journal
// files behind, which would quietly undercut the promise toki makes about keeping
// nothing it does not have to.
//
// The format is line-delimited JSON, which means a corrupt line costs one record
// rather than the file. That matters because the file is appended to outside the
// program's control: a machine losing power mid-write leaves a partial last line,
// and toki is a timer people leave running all day.
package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bishalr0y/toki/internal/config"
)

// File is the name records are kept in, inside the config directory.
const File = "sessions.jsonl"

// maxLine bounds a single record.
//
// The cap is not about defending against a hostile input — it is about a file
// that is not line-delimited at all. Without it, one line of binary garbage
// would be handed to a scanner that then tries to allocate a buffer for the rest
// of the file. Records are a few hundred bytes; this is far more than one needs
// and small enough to fail loudly rather than exhaust memory.
const maxLine = 1 << 16

// Record is one completed split.
//
// Only what actually happened is kept, never what was planned: the split's name
// and the figures it achieved. A record is therefore a statement about a session
// that ran, not a copy of the config, which can be edited afterwards without
// making the history wrong.
type Record struct {
	// Name is the split as it was configured, so the history still means
	// something after that entry is renamed or removed.
	Name string `json:"name"`
	// Focused is time actually spent in focus phases, which is less than the
	// split's nominal length whenever a phase was skipped.
	//
	// It is written to the file as whole seconds rather than as a time.Duration,
	// which marshals to nanoseconds. The field is named _s, and a file a person
	// might open should not quietly disagree with its own field name.
	Focused time.Duration `json:"-"`
	// Rounds is how many focus rounds finished, again short of the split's
	// cycle count if any were skipped.
	Rounds int `json:"rounds"`
	// EndedAt is when the split finished, in UTC. Stored as a time rather than
	// a date string so that reading and summing need no further parsing.
	EndedAt time.Time `json:"ended_at"`
}

// onDisk is the shape a record takes in the file, kept separate from Record so
// the two representations can differ without either being contorted.
type onDisk struct {
	Name     string `json:"name"`
	FocusedS int64  `json:"focused_s"`
	Rounds   int    `json:"rounds"`
	EndedAt  string `json:"ended_at"`
}

// MarshalJSON writes the record with Focused in whole seconds.
//
// Seconds are chosen over nanoseconds because the file is meant to be readable:
// a session is minutes and hours, and nobody reading `focused_s` should have to
// know that Go's duration is built on nanoseconds to work out what it means.
func (r Record) MarshalJSON() ([]byte, error) {
	// A session shorter than a second is not a thing, but rounding must not
	// produce a negative duration if it ever were, since the field is signed.
	seconds := max(int64(r.Focused/time.Second), 0)

	return json.Marshal(onDisk{
		Name:     r.Name,
		FocusedS: seconds,
		Rounds:   r.Rounds,
		EndedAt:  r.EndedAt.UTC().Format(time.RFC3339Nano),
	})
}

// UnmarshalJSON reads a record back.
//
// A record whose ended_at cannot be parsed is an error rather than a zero time,
// because a zero time would sort to 1970 and quietly fall outside every window —
// one unreadable record would vanish instead of being noticed.
func (r *Record) UnmarshalJSON(data []byte) error {
	var raw onDisk
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	ended, err := time.Parse(time.RFC3339Nano, raw.EndedAt)
	if err != nil {
		return fmt.Errorf("unreadable ended_at %q: %w", raw.EndedAt, err)
	}

	r.Name = raw.Name
	r.Focused = time.Duration(raw.FocusedS) * time.Second
	r.Rounds = raw.Rounds
	r.EndedAt = ended
	return nil
}

// Totals is a sum over some set of records.
type Totals struct {
	Focused time.Duration
	Rounds  int
	Splits  int
}

// Path is where records are kept: sessions.jsonl inside the config directory.
//
// It shares the directory rather than getting one of its own so that the whole
// of toki's state can be backed up or moved as a single folder, and so that
// removing it is a single familiar command.
func Path() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, File), nil
}

// Append adds one record, then prunes whatever has aged out.
//
// Pruning runs against the record's own EndedAt rather than the wall clock, so
// what "recent" means does not depend on when this happened to be called.
func Append(r Record) error {
	path, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Encoded before the file is opened, so a failure to marshal cannot leave a
	// half-written line behind. Record holds only a string, a duration and an int,
	// so in practice this cannot fail.
	line, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to encode history record: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open history file: %w", err)
	}

	// O_APPEND, so each write goes to the end of the file as it is at that moment
	// rather than at wherever it was when the file was opened. That matters
	// because the alternative is two toki instances interleaving into the same
	// offset and corrupting each other's records, which a plain O_WRONLY permits.
	//
	// Written in one call on purpose. A single write of a short buffer is not
	// guaranteed atomic by POSIX, but it is the most that can be asked of a plain
	// file without taking a lock, and every alternative is worse: a lock file
	// would need cleaning up, and an fsync per record would cost more than the
	// record is worth. A torn line is survivable anyway, because reading skips
	// what it cannot decode.
	if _, err := file.Write(append(line, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write history record: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close history file: %w", err)
	}

	// Only now, with the record safely on disk, and the error deliberately
	// discarded: a session that has been recorded must not be reported as failed
	// because tidying up afterwards did not work.
	_ = prune(path, r.EndedAt)

	return nil
}

// Since returns the records that ended at or after t, oldest first.
//
// Oldest first because that is the order the file is already in, so nothing has
// to be sorted. A caller displaying these wants the newest at the top and can
// reverse them; making that a property of the read would mean every caller had to
// know it was undone.
//
// A line that does not decode is skipped rather than reported. The likeliest
// cause is a half-written final line from a machine that lost power, and one
// unreadable record should not cost the user the other few hundred.
func Since(t time.Time) ([]Record, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	all, _, err := scan(path)
	if err != nil {
		return nil, err
	}

	out := make([]Record, 0, len(all))
	for _, r := range all {
		if !r.EndedAt.Before(t) {
			out = append(out, r)
		}
	}

	return out, nil
}

// scan reads the whole file, returning every record it can decode along with how
// many lines it had to skip.
//
// The skip count is what lets pruning notice a torn line. Reads deliberately
// ignore it — a caller wanting records should get the records — but a caller
// rewriting the file has to know something is being dropped.
func scan(path string) (records []Record, skipped int, err error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing recorded yet is an empty history, not a failure. A first
			// run should not have to distinguish "no sessions yet" from "broken".
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("failed to open history file: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), maxLine)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			skipped++
			continue
		}

		records = append(records, r)
	}

	if err := scanner.Err(); err != nil {
		// A line past maxLine, so the file is not the line-delimited JSON it
		// claims to be. Everything read before it is still good, so it is returned
		// alongside the error rather than thrown away.
		return records, skipped, fmt.Errorf("failed to read history file: %w", err)
	}

	return records, skipped, nil
}

// Today sums the records that ended today in now's own location.
//
// "Today" means since the local midnight before now, not the last twenty-four
// hours. A daily total that reset at a moving moment would be a number nobody
// could predict, and the point of looking at it is that it starts again at
// midnight.
//
// A location whose offset changed within the window — across a daylight saving
// boundary — gives the right answer anyway, because the two midnights are each
// computed in their own offset.
func Today(now time.Time) (Totals, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	records, err := Since(start)
	if err != nil {
		return Totals{}, err
	}

	return sum(records), nil
}

// Recent returns the records in the three calendar months ending at now, oldest
// first.
//
// Calendar months rather than ninety days: the caller asked for three months, and
// over a year the two drift far enough apart to be a different promise.
//
// Records dated after now are dropped, which needs a clock that moved backwards to
// matter — but a history that reaches into the future is not one to put on screen.
func Recent(now time.Time) ([]Record, error) {
	records, err := Since(now.AddDate(0, -RetentionMonths, 0))
	if err != nil {
		return nil, err
	}

	inWindow := records[:0]
	for _, r := range records {
		if !r.EndedAt.After(now) {
			inWindow = append(inWindow, r)
		}
	}

	return inWindow, nil
}

// Window sums the records in the three calendar months ending at now.
func Window(now time.Time) (Totals, error) {
	records, err := Recent(now)
	if err != nil {
		return Totals{}, err
	}

	return sum(records), nil
}

// sum totals a set of records.
func sum(records []Record) Totals {
	var t Totals
	for _, r := range records {
		t.Focused += r.Focused
		t.Rounds += r.Rounds
		t.Splits++
	}
	return t
}

// Day is one calendar day's records, with the totals for that day.
//
// It exists for --history, which prints days rather than a flat list: a session
// is identified by when it happened, and grouping is what makes a three month
// window readable rather than several hundred undated lines.
type Day struct {
	// Date is the day in the viewer's own location, as 2006-01-02. A string
	// sorts correctly, which is why it is not a time.Time.
	Date  string
	Total Totals
	Items []Record
}

// GroupByDay collects records into days, newest day first, and newest record
// first within each.
//
// Sorted rather than reversed. The records arrive oldest first, so reversing
// would be enough — but only for as long as that holds. A clock that moved
// backwards, or a file two toki instances wrote, leaves them out of order, and a
// reversed list would then put the newest session somewhere arbitrary. Sorting
// costs nothing at this size.
func GroupByDay(records []Record) []Day {
	if len(records) == 0 {
		return nil
	}

	byDate := make(map[string][]Record, 8)
	for _, r := range records {
		key := r.EndedAt.Format("2006-01-02")
		byDate[key] = append(byDate[key], r)
	}

	days := make([]Day, 0, len(byDate))
	for date, items := range byDate {
		ordered := append([]Record(nil), items...)
		slices.SortFunc(ordered, func(a, b Record) int {
			return b.EndedAt.Compare(a.EndedAt)
		})

		days = append(days, Day{Date: date, Total: sum(ordered), Items: ordered})
	}

	slices.SortFunc(days, func(a, b Day) int {
		return strings.Compare(b.Date, a.Date)
	})

	return days
}
