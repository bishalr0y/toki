// Package history records the splits you finished, so a session can be
// totalled up after the fact rather than only within one run of the timer.
//
// The file is plain JSON on purpose. It can be read, edited, deleted or moved to
// another machine without this program, and a corrupt file is something the user
// can see and fix rather than a database that needs its own tooling.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry is one split that ran to completion.
type Entry struct {
	// Split is the name of the split that ran, as it appeared in the config.
	Split string
	// StartedAt and EndedAt bound the whole session, breaks included.
	StartedAt time.Time
	EndedAt   time.Time
	// Focus is the time actually spent focusing. It is less than the session
	// length, because breaks do not count and neither does time spent paused.
	Focus time.Duration
	// Rounds is how many focus rounds finished.
	Rounds int
}

// Totals is the aggregate of a set of entries.
type Totals struct {
	// Splits is how many splits were finished.
	Splits int
	// Focus is their combined focus time.
	Focus time.Duration
}

// entry is the on-disk shape of an Entry.
//
// Focus is held as a whole number of seconds rather than a time.Duration,
// because time.Duration marshals as a nanosecond count and a file meant to be
// readable should not need a calculator to interpret.
type diskEntry struct {
	Split        string    `json:"split"`
	StartedAt    time.Time `json:"startedAt"`
	EndedAt      time.Time `json:"endedAt"`
	FocusSeconds int64     `json:"focusSeconds"`
	Rounds       int       `json:"rounds"`
}

func (e Entry) toDisk() diskEntry {
	return diskEntry{
		Split:        e.Split,
		StartedAt:    e.StartedAt,
		EndedAt:      e.EndedAt,
		FocusSeconds: int64(e.Focus.Round(time.Second) / time.Second),
		Rounds:       e.Rounds,
	}
}

func (d diskEntry) toEntry() Entry {
	return Entry{
		Split:     d.Split,
		StartedAt: d.StartedAt,
		EndedAt:   d.EndedAt,
		Focus:     time.Duration(d.FocusSeconds) * time.Second,
		Rounds:    d.Rounds,
	}
}

// Store is the history file's location on disk.
type Store struct {
	path string
}

// NewStore returns a Store backed by the file at path.
//
// The file is not touched until something is written, so this never fails and a
// Store is safe to build at start up.
func NewStore(path string) *Store { return &Store{path: path} }

// Append adds a finished split to the history.
//
// It rewrites the whole file, because the file is a JSON array and an append
// that has to read it anyway. Two toki processes writing at once would lose one
// of the two entries; that is acceptable for a single-user timer and cheaper
// than a lock file nobody would ever notice.
//
// The write goes to a temporary file and is renamed into place, so a crash or a
// full disk part way through leaves the previous history intact rather than a
// truncated file that can never be read again.
func (s *Store) Append(e Entry) error {
	existing, err := s.Entries()
	if err != nil {
		return err
	}

	written := make([]diskEntry, 0, len(existing)+1)
	for _, prev := range existing {
		written = append(written, prev.toDisk())
	}
	written = append(written, e.toDisk())

	data, err := json.MarshalIndent(written, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode history: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}

	// Renaming within the directory keeps the file on the same filesystem, so
	// the replacement is atomic.
	tmp, err := os.CreateTemp(dir, "history-*.json")
	if err != nil {
		return fmt.Errorf("failed to create a temporary history file: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once the rename below has succeeded

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write history: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write history: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("failed to set history permissions: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("failed to replace %s: %w", s.path, err)
	}

	return nil
}

// Entries returns every recorded split, oldest first.
//
// A file that does not exist yet, or exists but is empty, reads as no history:
// a first run has nothing to show and that is not a failure. A file that cannot
// be parsed is reported, because a silently empty history would make the totals
// quietly wrong and, since Append has to read the file before adding to it,
// would also stop every later split from being recorded.
func (s *Store) Entries() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", s.path, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var stored []diskEntry
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", s.path, err)
	}

	entries := make([]Entry, 0, len(stored))
	for _, e := range stored {
		entries = append(entries, e.toEntry())
	}
	return entries, nil
}

// TotalsSince sums the splits that ended at or after from.
//
// The caller supplies the boundary rather than the store reading a clock, in the
// same spirit as the timer taking now as an argument: that keeps the arithmetic
// checkable and leaves "today" to whoever knows the timezone. The boundary is
// inclusive, so a split finishing exactly at midnight belongs to the new day
// rather than vanishing from both.
func (s *Store) TotalsSince(from time.Time) (Totals, error) {
	entries, err := s.Entries()
	if err != nil {
		return Totals{}, err
	}

	var totals Totals
	for _, e := range entries {
		if e.EndedAt.Before(from) {
			continue
		}
		totals.Splits++
		totals.Focus += e.Focus
	}

	return totals, nil
}
