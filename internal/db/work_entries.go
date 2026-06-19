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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lydietoure/orbit/internal/core"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrWorkEntryNotFound is returned by [GetWorkEntry] when no row
// matches the requested ID. Use errors.Is to detect it.
var ErrWorkEntryNotFound = errors.New("work entry not found")

// ErrWorkEntryTitleTaken is returned by [InsertWorkEntry] when the
// UNIQUE COLLATE NOCASE constraint on work_entries.title rejects the
// insert because another entry already uses that title (case-
// insensitive match). Use errors.Is to detect it.
var ErrWorkEntryTitleTaken = errors.New("a work entry with that title already exists")

// workEntryColumnList is the canonical column order for SELECTs that
// map onto a core.WorkEntry. Order must stay in lockstep with
// scanWorkEntry; both [workEntryColumns] and [prefixedWorkEntryColumns]
// derive from this single source of truth.
var workEntryColumnList = []string{
	"id", "title", "description", "status",
	"status_reason", "pad_path", "created_at", "updated_at",
}

// workEntryColumns is the unqualified comma-joined column list,
// e.g. for `SELECT <columns> FROM work_entries`.
var workEntryColumns = strings.Join(workEntryColumnList, ", ")

// prefixedWorkEntryColumns returns the column list qualified by the
// given table alias (e.g. `w.id, w.title, ...`). Needed for JOINs
// where unqualified `id` would be ambiguous.
func prefixedWorkEntryColumns(alias string) string {
	out := make([]string, len(workEntryColumnList))
	for i, c := range workEntryColumnList {
		out[i] = alias + "." + c
	}
	return strings.Join(out, ", ")
}

// rowScanner is the subset of *sql.Row / *sql.Rows that scanWorkEntry
// needs. Lets us share one mapping helper between single-row reads
// (GetWorkEntry) and result-set reads (ListWorkEntries).
type rowScanner interface {
	Scan(dest ...any) error
}

// scanWorkEntry reads one row in workEntryColumns order into a
// core.WorkEntry. NULL TEXT columns come back as empty strings (the
// inverse of nullableText), and timestamps are parsed from RFC3339Nano.
func scanWorkEntry(s rowScanner) (core.WorkEntry, error) {
	var (
		e                          core.WorkEntry
		status                     string
		desc, reason, pad          sql.NullString
		createdAtStr, updatedAtStr string
	)
	if err := s.Scan(
		&e.ID, &e.Title, &desc, &status, &reason, &pad, &createdAtStr, &updatedAtStr,
	); err != nil {
		return core.WorkEntry{}, err
	}
	e.Description = desc.String
	e.Status = core.WorkEntryStatus(status)
	e.StatusReason = reason.String
	e.PadPath = pad.String

	createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return core.WorkEntry{}, fmt.Errorf("parse created_at %q: %w", createdAtStr, err)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtStr)
	if err != nil {
		return core.WorkEntry{}, fmt.Errorf("parse updated_at %q: %w", updatedAtStr, err)
	}
	e.CreatedAt = createdAt
	e.UpdatedAt = updatedAt
	return e, nil
}

// InsertWorkEntry persists a fully-constructed [core.WorkEntry]. It
// performs no validation or defaulting — that is
// [core.NewWorkEntry]'s job — so the caller must hand in a record
// that is already valid and complete (ID, Status, CreatedAt,
// UpdatedAt all set).
func InsertWorkEntry(ctx context.Context, db *sql.DB, e core.WorkEntry) error {
	const stmt = `INSERT INTO work_entries
		(id, title, description, status, status_reason, pad_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.ExecContext(ctx, stmt,
		e.ID,
		e.Title,
		nullableText(e.Description),
		string(e.Status),
		nullableText(e.StatusReason),
		nullableText(e.PadPath),
		isoTime(e.CreatedAt),
		isoTime(e.UpdatedAt),
	)
	if err != nil {
		if isUniqueTitleViolation(err) {
			return fmt.Errorf("%w: %q", ErrWorkEntryTitleTaken, e.Title)
		}
		return fmt.Errorf("insert work entry: %w", err)
	}
	return nil
}

// isUniqueTitleViolation reports whether err is a SQLite UNIQUE
// constraint failure specifically on work_entries.title. Other UNIQUE
// failures (e.g., PRIMARY KEY collision on id) take a different
// extended code (SQLITE_CONSTRAINT_PRIMARYKEY) and pass through
// unchanged so they read as the generic insert error they are.
func isUniqueTitleViolation(err error) bool {
	var serr *sqlite.Error
	if !errors.As(err, &serr) {
		return false
	}
	if serr.Code() != sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return false
	}
	// The driver's message embeds the column. Matching on it scopes
	// the sentinel to title violations specifically; a future UNIQUE
	// on another column won't get silently mis-translated.
	return strings.Contains(serr.Error(), "work_entries.title")
}

// GetWorkEntry returns the work entry with the given ID. If no such
// entry exists, the returned error wraps [ErrWorkEntryNotFound].
// The returned entry's Tags slice is populated via a follow-up query
// (alphabetical; nil if untagged).
func GetWorkEntry(ctx context.Context, db *sql.DB, id string) (core.WorkEntry, error) {
	stmt := `SELECT ` + workEntryColumns + ` FROM work_entries WHERE id = ?`
	row := db.QueryRowContext(ctx, stmt, id)
	e, err := scanWorkEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.WorkEntry{}, fmt.Errorf("%w: %s", ErrWorkEntryNotFound, id)
	}
	if err != nil {
		return core.WorkEntry{}, fmt.Errorf("get work entry %s: %w", id, err)
	}
	tags, err := ListTagsForWorkEntry(ctx, db, e.ID)
	if err != nil {
		return core.WorkEntry{}, err
	}
	e.Tags = tags
	return e, nil
}

// ListWorkEntries returns every work entry, most recently created
// first. An empty table is not an error — the returned slice is
// simply empty. Each entry's Tags slice is loaded via a follow-up
// query; N+1 is fine at M0 scale.
func ListWorkEntries(ctx context.Context, db *sql.DB) ([]core.WorkEntry, error) {
	stmt := `SELECT ` + workEntryColumns + ` FROM work_entries ORDER BY created_at DESC, id DESC`
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("list work entries: %w", err)
	}
	defer rows.Close()

	var out []core.WorkEntry
	for rows.Next() {
		e, err := scanWorkEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("list work entries: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list work entries: %w", err)
	}
	for i := range out {
		tags, err := ListTagsForWorkEntry(ctx, db, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Tags = tags
	}
	return out, nil
}

// DeleteWorkEntry removes the work_entries row with the given id.
// Schema-level cascades take care of the side effects:
//   - work_entry_tags rows referencing this entry are deleted
//     (ON DELETE CASCADE).
//   - state.selected_work_entry_id is set to NULL if it pointed
//     here (ON DELETE SET NULL).
//
// The shared `tags` vocabulary is intentionally NOT touched —
// tag rows are reused across entries and an orphaned tag is just
// an unused label, not garbage.
//
// Returns a wrapped [ErrWorkEntryNotFound] if no row matched, so
// callers can distinguish "already gone" from a real failure.
func DeleteWorkEntry(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM work_entries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete work entry %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete work entry %s: rows affected: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", ErrWorkEntryNotFound, id)
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
