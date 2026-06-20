package app

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
)

// Use cases behind `orbit work project` and `orbit work owner`. These
// are thin conveniences over the tag layer that also enforce the
// cardinality rules from docs/DATA_MODEL.md: `project:*` may appear
// multiple times, `owner:*` at most once per entry.
//
// All reserved tags are stored as ordinary `project:<name>` /
// `owner:<name>` tags via the existing tag gateway, so they round-trip
// through `work tag`, `work show`, exports, etc. unchanged.

// AddProject attaches a `project:<name>` tag to a work entry. Multiple
// projects per entry are allowed; re-adding the same project is a
// no-op (idempotent). The bare project name is returned alongside the
// resolved entry id so callers can echo exactly what was mutated.
//
// An empty id falls back to the currently selected entry (returns
// [ErrNoTargetWorkEntry] if nothing is selected). Wraps
// [db.ErrWorkEntryNotFound] if the entry does not exist.
func AddProject(ctx context.Context, id, rawName string) (resolvedID, project string, err error) {
	tag, err := core.ProjectTagName(rawName)
	if err != nil {
		return "", "", err
	}

	d, closer, err := open()
	if err != nil {
		return "", "", err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", "", err
	}
	if _, err := db.GetWorkEntry(ctx, d, target); err != nil {
		return "", "", err
	}
	if err := db.AddTagToWorkEntry(ctx, d, target, tag); err != nil {
		return "", "", err
	}
	return target, strings.TrimPrefix(tag, core.ProjectTagPrefix), nil
}

// RemoveProject drops a `project:<name>` tag from a work entry.
// Returns the resolved id and bare project name. Wraps
// [db.ErrTagNotOnEntry] when the project is not on the entry,
// [db.ErrWorkEntryNotFound] when the entry is missing, and
// [ErrNoTargetWorkEntry] when no id was given and nothing is selected.
func RemoveProject(ctx context.Context, id, rawName string) (resolvedID, project string, err error) {
	tag, err := core.ProjectTagName(rawName)
	if err != nil {
		return "", "", err
	}

	d, closer, err := open()
	if err != nil {
		return "", "", err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", "", err
	}
	if _, err := db.GetWorkEntry(ctx, d, target); err != nil {
		return "", "", err
	}
	if err := db.RemoveTagFromWorkEntry(ctx, d, target, tag); err != nil {
		return "", "", err
	}
	return target, strings.TrimPrefix(tag, core.ProjectTagPrefix), nil
}

// ListProjects returns the bare project names on a work entry, sorted
// (the tag layer reads tags alphabetically). An entry with no projects
// yields a nil slice, not an error.
//
// An empty id falls back to the currently selected entry; wraps
// [db.ErrWorkEntryNotFound] / [ErrNoTargetWorkEntry] as the other
// reserved-tag use cases do.
func ListProjects(ctx context.Context, id string) (resolvedID string, projects []string, err error) {
	d, closer, err := open()
	if err != nil {
		return "", nil, err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", nil, err
	}
	entry, err := db.GetWorkEntry(ctx, d, target)
	if err != nil {
		return "", nil, err
	}
	projects, _, _ = core.PartitionReservedTags(entry.Tags)
	return target, projects, nil
}

// SetOwner sets the `owner:<name>` tag on a work entry, replacing any
// owner already present so the single-owner cardinality from
// docs/DATA_MODEL.md holds. Re-setting to the same value is a no-op;
// the result is always exactly one `owner:*` tag. Returns the resolved
// id and the bare owner name.
//
// The remove-then-add is not wrapped in a transaction (the app package
// has no tx helper yet — see the note on [CreateWork]); for orbit's
// single-user, no-daemon model the interleaving window is irrelevant.
//
// An empty id falls back to the currently selected entry. Wraps
// [db.ErrWorkEntryNotFound] / [ErrNoTargetWorkEntry].
func SetOwner(ctx context.Context, id, rawName string) (resolvedID, owner string, err error) {
	tag, err := core.OwnerTagName(rawName)
	if err != nil {
		return "", "", err
	}

	d, closer, err := open()
	if err != nil {
		return "", "", err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", "", err
	}
	entry, err := db.GetWorkEntry(ctx, d, target)
	if err != nil {
		return "", "", err
	}

	// Drop any existing owner tags first so setting is a swap, never
	// an accumulation. Skip the one we're about to (re)add to keep
	// the operation idempotent and avoid a needless delete/insert.
	if err := removeOwnerTags(ctx, d, target, entry.Tags, tag); err != nil {
		return "", "", err
	}
	if err := db.AddTagToWorkEntry(ctx, d, target, tag); err != nil {
		return "", "", err
	}
	return target, strings.TrimPrefix(tag, core.OwnerTagPrefix), nil
}

// ClearOwner removes the `owner:*` tag from a work entry, if any.
// Returns the bare owner name that was cleared ("" if the entry had no
// owner). Clearing an already-ownerless entry is a no-op, not an
// error.
//
// An empty id falls back to the currently selected entry. Wraps
// [db.ErrWorkEntryNotFound] / [ErrNoTargetWorkEntry].
func ClearOwner(ctx context.Context, id string) (resolvedID, owner string, err error) {
	d, closer, err := open()
	if err != nil {
		return "", "", err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", "", err
	}
	entry, err := db.GetWorkEntry(ctx, d, target)
	if err != nil {
		return "", "", err
	}
	_, prev, _ := core.PartitionReservedTags(entry.Tags)
	if err := removeOwnerTags(ctx, d, target, entry.Tags, ""); err != nil {
		return "", "", err
	}
	return target, prev, nil
}

// GetOwner returns the bare owner name on a work entry ("" if none).
//
// An empty id falls back to the currently selected entry. Wraps
// [db.ErrWorkEntryNotFound] / [ErrNoTargetWorkEntry].
func GetOwner(ctx context.Context, id string) (resolvedID, owner string, err error) {
	d, closer, err := open()
	if err != nil {
		return "", "", err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", "", err
	}
	entry, err := db.GetWorkEntry(ctx, d, target)
	if err != nil {
		return "", "", err
	}
	_, owner, _ = core.PartitionReservedTags(entry.Tags)
	return target, owner, nil
}

// removeOwnerTags deletes every `owner:*` tag in tags from the entry,
// except keep (when non-empty) which the caller intends to retain.
// tags is the entry's current tag list (already loaded by the caller).
func removeOwnerTags(ctx context.Context, d *sql.DB, workEntryID string, tags []string, keep string) error {
	for _, t := range tags {
		if !strings.HasPrefix(t, core.OwnerTagPrefix) || t == keep {
			continue
		}
		if err := db.RemoveTagFromWorkEntry(ctx, d, workEntryID, t); err != nil {
			return err
		}
	}
	return nil
}
