package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RetentionMonths is how far back records are kept.
//
// Three calendar months rather than ninety days, so the window matches the one
// --history shows: a record cannot be pruned away while still being something
// the history screen claims to be showing.
const RetentionMonths = 3

// prune drops records that have aged out, and any line it could not decode.
//
// It rewrites the whole file rather than editing it. At three months of one
// record per session the file is a few hundred lines, so a full rewrite is
// free, and it means there is exactly one code path that writes the file
// instead of two — one appends and one rewrites, which is two chances to get
// the format subtly wrong.
//
// Pruning is best effort by design. A failure here costs disk and nothing else:
// the stale records stay readable, and the next attempt prunes them again. That
// is a different bargain from a failed append, which loses a session outright,
// so Append discards this error on purpose.
func prune(path string, now time.Time) error {
	records, skipped, err := scan(path)
	if err != nil {
		return err
	}

	cutoff := now.AddDate(0, -RetentionMonths, 0)

	kept := make([]Record, 0, len(records))
	for _, r := range records {
		// A record exactly on the cutoff is kept, so the window --history shows
		// and the window that is kept are the same window.
		if r.EndedAt.Before(cutoff) {
			continue
		}
		kept = append(kept, r)
	}

	// Nothing to do. The common case, since pruning only has work to do once
	// something has actually aged out.
	if len(kept) == len(records) && skipped == 0 {
		return nil
	}

	return rewrite(path, kept)
}

// rewrite replaces the file with the given records.
//
// The new content goes to a temporary file in the same directory and is then
// renamed over the original. Rename within a directory is atomic, so a machine
// that loses power mid-prune finds either the old file or the new one and never
// a half-written mixture of the two — which, for a file the user may have no
// backup of, is the whole reason for doing it this way rather than truncating
// and rewriting in place.
func rewrite(path string, records []Record) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, File+".*")
	if err != nil {
		return fmt.Errorf("failed to create temporary history file: %w", err)
	}
	// Renamed into place or removed. Either way it must not survive the call, or
	// an interrupted prune would leave litter next to the file it was pruning.
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			_ = tmp.Close()
			return fmt.Errorf("failed to encode history record: %w", err)
		}

		if _, err := tmp.Write(append(line, '\n')); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("failed to write history record: %w", err)
		}
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary history file: %w", err)
	}

	// CreateTemp makes the file 0600. The history was 0644 and lives beside a
	// config that is world readable, so the mode is restored before the rename
	// rather than after: the moment the file is named it is the real one.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("failed to set history file mode: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to replace history file: %w", err)
	}

	return nil
}
