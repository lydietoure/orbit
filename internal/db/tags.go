package db

// Storage gateway for the tags and work_entry_tags tables. Same
// layering rules as work_entries.go / state.go: validation lives in
// core (see [core.NormalizeTagName]); this layer assumes the names
// it gets have already been normalized.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lydietoure/orbit/internal/core"
)

// ErrTagNotOnEntry is returned by [RemoveTagFromWorkEntry] when the
// requested association doesn't exist — either because the tag has
// never been used or because the given work entry doesn't have it.
// Detect with errors.Is.
var ErrTagNotOnEntry = errors.New("tag is not on this work entry")

// EnsureTag returns the row id of the tag with the given name,
// inserting it first if no such row exists yet. Idempotent. The
// caller must supply an already-normalized name.
func EnsureTag(ctx context.Context, db *sql.DB, name string) (int64, error) {
	if _, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO tags (name) VALUES (?)`, name); err != nil {
		return 0, fmt.Errorf("ensure tag %q: %w", name, err)
	}
	var id int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM tags WHERE name = ?`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup tag %q: %w", name, err)
	}
	return id, nil
}

// AddTagToWorkEntry creates the tag if needed and associates it
// with the given work entry. Idempotent — re-tagging is a no-op
// thanks to the join table's PRIMARY KEY (work_entry_id, tag_id).
//
// If workEntryID does not refer to an existing entry, the FK
// constraint will reject the insert; callers that want a clean
// [ErrWorkEntryNotFound] should validate the id up front (the app
// layer does this).
func AddTagToWorkEntry(ctx context.Context, db *sql.DB, workEntryID, name string) error {
	tagID, err := EnsureTag(ctx, db, name)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO work_entry_tags (work_entry_id, tag_id) VALUES (?, ?)`,
		workEntryID, tagID,
	); err != nil {
		return fmt.Errorf("tag work entry %s with %q: %w", workEntryID, name, err)
	}
	return nil
}

// RemoveTagFromWorkEntry deletes the association between workEntryID
// and the named tag. Returns [ErrTagNotOnEntry] if no association
// existed. The tag row itself is left in place even if no other
// entries use it — orphan cleanup is a separate concern.
func RemoveTagFromWorkEntry(ctx context.Context, db *sql.DB, workEntryID, name string) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM work_entry_tags
		 WHERE work_entry_id = ?
		   AND tag_id = (SELECT id FROM tags WHERE name = ?)`,
		workEntryID, name,
	)
	if err != nil {
		return fmt.Errorf("untag work entry %s from %q: %w", workEntryID, name, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("untag work entry %s from %q: %w", workEntryID, name, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s / %q", ErrTagNotOnEntry, workEntryID, name)
	}
	return nil
}

// ListTagsForWorkEntry returns the tag names associated with the
// given work entry in alphabetical order. An entry with no tags
// returns a nil slice (not an error).
func ListTagsForWorkEntry(ctx context.Context, db *sql.DB, workEntryID string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT t.name FROM tags t
		 JOIN work_entry_tags wet ON wet.tag_id = t.id
		 WHERE wet.work_entry_id = ?
		 ORDER BY t.name`,
		workEntryID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tags for %s: %w", workEntryID, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("list tags for %s: %w", workEntryID, err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tags for %s: %w", workEntryID, err)
	}
	return out, nil
}

// ListAllTags returns every tag that is attached to at least one work
// entry, paired with the number of entries carrying it, in
// alphabetical order by name. Orphaned tags — rows in `tags` with no
// remaining associations — are omitted, since a usage overview of
// unused labels is just noise. An empty vocabulary returns a nil slice
// (not an error).
func ListAllTags(ctx context.Context, db *sql.DB) ([]core.TagCount, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT t.name, COUNT(wet.work_entry_id) AS n
		 FROM tags t
		 JOIN work_entry_tags wet ON wet.tag_id = t.id
		 GROUP BY t.id
		 ORDER BY t.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all tags: %w", err)
	}
	defer rows.Close()

	var out []core.TagCount
	for rows.Next() {
		var tc core.TagCount
		if err := rows.Scan(&tc.Name, &tc.Count); err != nil {
			return nil, fmt.Errorf("list all tags: %w", err)
		}
		out = append(out, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list all tags: %w", err)
	}
	return out, nil
}
