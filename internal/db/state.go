package db

// This file is the storage gateway for the `state` table (the
// singleton row at id=1 that tracks selected work entry and any
// other per-install bookkeeping). Same layering as work_entries.go:
// no validation, no business rules — callers hand us values that
// are already valid.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lydietoure/orbit/internal/core"
)

// ErrNoSelectedEntry is returned by [GetSelectedWorkEntry] when the
// state singleton has no selected work entry (the default state, and
// also the state after [ForgetSelectedWorkEntry]). Detect with
// errors.Is.
var ErrNoSelectedEntry = errors.New("no work entry is currently selected")

// SelectWorkEntry sets the selected work entry in the state
// singleton. The caller must ensure id refers to an existing
// work_entries row; if it does not, the foreign-key constraint will
// reject the UPDATE.
//
// Idempotent: selecting the same id twice is a no-op. Selecting a
// different id overwrites the previous selection.
func SelectWorkEntry(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE state SET selected_work_entry_id = ? WHERE id = 1`, id)
	if err != nil {
		return fmt.Errorf("select work entry %s: %w", id, err)
	}
	return nil
}

// ForgetSelectedWorkEntry clears the selected work entry. Safe to
// call when nothing is currently selected — the UPDATE is a no-op
// in that case.
func ForgetSelectedWorkEntry(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx,
		`UPDATE state SET selected_work_entry_id = NULL WHERE id = 1`)
	if err != nil {
		return fmt.Errorf("forget selected work entry: %w", err)
	}
	return nil
}

// GetSelectedWorkEntry returns the currently selected work entry, or
// [ErrNoSelectedEntry] if none is selected. A single JOIN pulls the
// entry in one round trip rather than two queries.
func GetSelectedWorkEntry(ctx context.Context, db *sql.DB) (core.WorkEntry, error) {
	stmt := `SELECT ` + prefixedWorkEntryColumns("w") + `
		FROM state s
		JOIN work_entries w ON w.id = s.selected_work_entry_id
		WHERE s.id = 1`
	row := db.QueryRowContext(ctx, stmt)
	e, err := scanWorkEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.WorkEntry{}, ErrNoSelectedEntry
	}
	if err != nil {
		return core.WorkEntry{}, fmt.Errorf("get selected work entry: %w", err)
	}
	return e, nil
}
