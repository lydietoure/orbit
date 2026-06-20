package db

// Storage gateway for the notes table. Same layering rules as
// artifacts.go: validation (path absolutization, date format) lives in
// the core/app layers; this layer just runs SQL.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lydietoure/orbit/internal/core"
)

// ErrNoteNotOnEntry is returned by [RemoveNote] when the work entry has
// no note at the given path. Detect with errors.Is.
var ErrNoteNotOnEntry = errors.New("note is not on this work entry")

// AddNote links a [core.Note] to a work entry. Idempotent: the notes
// table's UNIQUE(work_entry_id, path, date) constraint turns a
// duplicate (same file, same date) into a no-op via INSERT OR IGNORE.
// The same file on a different date is a distinct note.
//
// If n.WorkEntryID does not refer to an existing entry the FK
// constraint rejects the insert; the app layer validates the id up
// front to surface a clean [ErrWorkEntryNotFound].
func AddNote(ctx context.Context, db *sql.DB, n core.Note) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO notes (work_entry_id, path, date, created_at)
		 VALUES (?, ?, ?, ?)`,
		n.WorkEntryID, n.Path, n.Date, isoTime(n.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("link note to %s: %w", n.WorkEntryID, err)
	}
	return nil
}

// RemoveNote deletes every note at the given path from the work entry,
// regardless of date. Returns [ErrNoteNotOnEntry] if the entry had no
// note at that path.
func RemoveNote(ctx context.Context, db *sql.DB, workEntryID, path string) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM notes WHERE work_entry_id = ? AND path = ?`,
		workEntryID, path,
	)
	if err != nil {
		return fmt.Errorf("unlink note from %s: %w", workEntryID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("unlink note from %s: %w", workEntryID, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s / %q", ErrNoteNotOnEntry, workEntryID, path)
	}
	return nil
}

// ListNotesForWorkEntry returns the notes linked to the given work
// entry, newest logical date first (ties broken by id). An entry with
// no notes returns a nil slice, not an error.
func ListNotesForWorkEntry(ctx context.Context, db *sql.DB, workEntryID string) ([]core.Note, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, work_entry_id, path, date, created_at
		 FROM notes WHERE work_entry_id = ?
		 ORDER BY date DESC, id`,
		workEntryID,
	)
	if err != nil {
		return nil, fmt.Errorf("list notes for %s: %w", workEntryID, err)
	}
	defer rows.Close()

	var out []core.Note
	for rows.Next() {
		var (
			n            core.Note
			createdAtStr string
		)
		if err := rows.Scan(&n.ID, &n.WorkEntryID, &n.Path, &n.Date, &createdAtStr); err != nil {
			return nil, fmt.Errorf("list notes for %s: %w", workEntryID, err)
		}
		createdAt, err := parseTime(createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("list notes for %s: %w", workEntryID, err)
		}
		n.CreatedAt = createdAt
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list notes for %s: %w", workEntryID, err)
	}
	return out, nil
}
