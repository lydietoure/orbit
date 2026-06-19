package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lydietoure/orbit/internal/core"
)

// CreateWorkEntryParams holds the user-supplied input for a new work
// entry. Computed fields (ID, timestamps) are filled in by
// [CreateWorkEntry], not the caller.
type CreateWorkEntryParams struct {
	// Title is required and trimmed of surrounding whitespace.
	Title string
	// Description is an optional longer explanation. Empty means absent.
	Description string
	// Status is the initial lifecycle status. Defaults to
	// [core.StatusNew] when zero.
	Status core.WorkEntryStatus
	// StatusReason explains the status. Required when Status is
	// [core.StatusAbandoned]; otherwise optional.
	StatusReason string
	// ScratchpadPath is an optional filesystem path to scratch work.
	// Empty means absent.
	ScratchpadPath string
}

// CreateWorkEntry inserts a new work entry into the database and
// returns the persisted record with ID, status, and timestamps filled
// in. The ID is generated via [core.NewID]; CreatedAt and UpdatedAt
// are set to the current time (UTC) and are equal at insert time.
//
// Validation errors (empty title, invalid status, missing reason for
// abandoned) are returned before any DB call is made.
func CreateWorkEntry(ctx context.Context, db *sql.DB, p CreateWorkEntryParams) (core.WorkEntry, error) {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		return core.WorkEntry{}, errors.New("work entry title is required")
	}

	status := p.Status
	if status == "" {
		status = core.StatusNew
	}
	if !status.Valid() {
		return core.WorkEntry{}, fmt.Errorf("invalid work entry status %q", status)
	}
	if status == core.StatusAbandoned && strings.TrimSpace(p.StatusReason) == "" {
		return core.WorkEntry{}, errors.New("status reason is required when status is abandoned")
	}

	now := time.Now().UTC()
	entry := core.WorkEntry{
		ID:             core.NewID(),
		Title:          title,
		Description:    p.Description,
		Status:         status,
		StatusReason:   p.StatusReason,
		ScratchpadPath: p.ScratchpadPath,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	const stmt = `INSERT INTO work_entries
		(id, title, description, status, status_reason, scratchpad_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.ExecContext(ctx, stmt,
		entry.ID,
		entry.Title,
		nullableText(entry.Description),
		string(entry.Status),
		nullableText(entry.StatusReason),
		nullableText(entry.ScratchpadPath),
		isoTime(entry.CreatedAt),
		isoTime(entry.UpdatedAt),
	)
	if err != nil {
		return core.WorkEntry{}, fmt.Errorf("insert work entry: %w", err)
	}
	return entry, nil
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
