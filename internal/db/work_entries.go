package db

// This file is the storage gateway for the work_entries table.
// Every read/write of that table goes through a function defined
// here — callers (cli/, future packages) never write SQL directly.
//
// Layering:
//   core/  — defines what a WorkEntry is and what makes one valid
//            (the struct, the status enum, NewWorkEntry). Pure Go,
//            no *sql.DB, no I/O.
//   db/    — this layer. Takes already-valid core.WorkEntry values
//            (or query parameters) and runs SQL. No validation, no
//            business rules — if it got here, it's assumed good.
//   cli/   — wires the two together: parses flags, calls core to
//            build, calls db to persist, prints output.
//
// Naming convention: this file is named after the table, not after
// a single operation. As we add Get/List/Update/Delete they all
// land in here until the file gets unwieldy. Same for the _test file.

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lydietoure/orbit/internal/core"
)

// InsertWorkEntry persists a fully-constructed [core.WorkEntry]. It
// performs no validation or defaulting — that is
// [core.NewWorkEntry]'s job — so the caller must hand in a record
// that is already valid and complete (ID, Status, CreatedAt,
// UpdatedAt all set).
func InsertWorkEntry(ctx context.Context, db *sql.DB, e core.WorkEntry) error {
	const stmt = `INSERT INTO work_entries
		(id, title, description, status, status_reason, scratchpad_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.ExecContext(ctx, stmt,
		e.ID,
		e.Title,
		nullableText(e.Description),
		string(e.Status),
		nullableText(e.StatusReason),
		nullableText(e.ScratchpadPath),
		isoTime(e.CreatedAt),
		isoTime(e.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert work entry: %w", err)
	}
	return nil
}

// nullableText returns nil for the empty string and s otherwise. Passing
// nil to a SQL driver writes NULL; passing an empty string writes ”.
// Using NULL keeps optional fields cleanly absent.
func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// isoTime formats t as RFC3339Nano in UTC for storage in TEXT columns.
// Lexical order on the resulting strings agrees with chronological
// order, so plain SQL ORDER BY works as expected.
func isoTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
