package db

// Storage gateway for the artifacts table. Same layering rules as
// tags.go / work_entries.go: validation lives in core (see
// [core.ArtifactType.NormalizeValue]); this layer assumes the values
// it gets are already valid and just runs SQL.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lydietoure/orbit/internal/core"
)

// ErrArtifactNotOnEntry is returned by [RemoveArtifact] when no
// matching artifact exists on the given work entry. Detect with
// errors.Is.
var ErrArtifactNotOnEntry = errors.New("artifact is not on this work entry")

// AddArtifact links a [core.Artifact] to a work entry. Idempotent:
// the artifacts table's UNIQUE(work_entry_id, type, value) constraint
// turns a duplicate link into a no-op via INSERT OR IGNORE.
//
// If a.WorkEntryID does not refer to an existing entry the FK
// constraint rejects the insert; callers that want a clean
// [ErrWorkEntryNotFound] should validate the id up front (the app
// layer does this).
func AddArtifact(ctx context.Context, db *sql.DB, a core.Artifact) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO artifacts (work_entry_id, type, value, created_at)
		 VALUES (?, ?, ?, ?)`,
		a.WorkEntryID, string(a.Type), a.Value, isoTime(a.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("link %s artifact to %s: %w", a.Type, a.WorkEntryID, err)
	}
	return nil
}

// RemoveArtifact deletes the artifact of the given type and value from
// the work entry. Returns [ErrArtifactNotOnEntry] if no such artifact
// existed.
func RemoveArtifact(ctx context.Context, db *sql.DB, workEntryID string, t core.ArtifactType, value string) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM artifacts WHERE work_entry_id = ? AND type = ? AND value = ?`,
		workEntryID, string(t), value,
	)
	if err != nil {
		return fmt.Errorf("unlink %s artifact from %s: %w", t, workEntryID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("unlink %s artifact from %s: %w", t, workEntryID, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s / %s %q", ErrArtifactNotOnEntry, workEntryID, t, value)
	}
	return nil
}

// ListArtifactsForWorkEntry returns the artifacts linked to the given
// work entry, oldest first (then by id for stable ordering). An entry
// with no artifacts returns a nil slice, not an error.
func ListArtifactsForWorkEntry(ctx context.Context, db *sql.DB, workEntryID string) ([]core.Artifact, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, work_entry_id, type, value, created_at
		 FROM artifacts WHERE work_entry_id = ?
		 ORDER BY created_at, id`,
		workEntryID,
	)
	if err != nil {
		return nil, fmt.Errorf("list artifacts for %s: %w", workEntryID, err)
	}
	defer rows.Close()

	var out []core.Artifact
	for rows.Next() {
		var (
			a            core.Artifact
			typeStr      string
			createdAtStr string
		)
		if err := rows.Scan(&a.ID, &a.WorkEntryID, &typeStr, &a.Value, &createdAtStr); err != nil {
			return nil, fmt.Errorf("list artifacts for %s: %w", workEntryID, err)
		}
		a.Type = core.ArtifactType(typeStr)
		createdAt, err := parseTime(createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("list artifacts for %s: %w", workEntryID, err)
		}
		a.CreatedAt = createdAt
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list artifacts for %s: %w", workEntryID, err)
	}
	return out, nil
}
