package app

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
)

// ErrNoTargetWorkEntry is returned by use cases that accept an
// optional id when no id was supplied AND no entry is currently
// selected — i.e. there is no work entry to operate on. Surfaced
// as the error from `orbit work show` / `work tag` etc. when run
// with no id and an empty selection.
var ErrNoTargetWorkEntry = errors.New(
	"no work entry id given and no entry is currently selected; " +
		"pass an id, run `orbit work select <id>`, or `orbit work list` to find one",
)

// resolveTargetID returns id if non-empty, otherwise the id of the
// currently selected entry. If neither is available it returns
// [ErrNoTargetWorkEntry] so commands surface a single, helpful
// message instead of leaking the lower-level [db.ErrNoSelectedEntry].
func resolveTargetID(ctx context.Context, d *sql.DB, id string) (string, error) {
	if id != "" {
		return id, nil
	}
	sel, err := db.GetSelectedWorkEntry(ctx, d)
	if errors.Is(err, db.ErrNoSelectedEntry) {
		return "", ErrNoTargetWorkEntry
	}
	if err != nil {
		return "", err
	}
	return sel.ID, nil
}

// CreateWorkParams is the input to [CreateWork]. Mirrors the cli flag
// set; the use case doesn't care where the values came from.
type CreateWorkParams struct {
	Title          string
	Description    string
	ScratchpadPath string
	// Tags is the optional list of tag names to attach. Each is
	// normalized (lower-case, trim) via [core.NormalizeTagName]
	// before insert; duplicates after normalization are deduped.
	Tags []string
	// NoSelect skips the auto-select step after insert. Default
	// behavior (NoSelect == false) is to select the freshly created
	// entry so subsequent `orbit work` commands operate on it without
	// the user having to copy the ID around. Scripts that create many
	// entries in a row want NoSelect == true.
	NoSelect bool
}

// CreateWork is the use case behind `orbit work new`: build a
// validated [core.WorkEntry] from the params, persist it, attach
// any requested tags, and (unless p.NoSelect) make it the currently
// selected entry.
//
// Auto-select also promotes the initial status to [core.StatusInProgress]
// — the user is creating the entry to start working on it right now.
// With NoSelect the entry is born [core.StatusNew] (queued for later)
// and only `orbit work` lifecycle commands move it forward. Note that
// the plain `orbit work select` command does NOT do this promotion;
// only auto-select via `work new` does.
//
// Tag names are normalized and validated up front so an invalid tag
// is rejected before any DB writes happen. After normalization,
// duplicate tag names are silently deduped.
//
// Insert + tag + select are NOT transactional — if a later step
// fails the entry still exists and the user can recover with the
// targeted command (`work tag`, `work select`). Worth revisiting
// once the db package grows a tx helper.
func CreateWork(ctx context.Context, p CreateWorkParams) (core.WorkEntry, error) {
	tags, err := normalizeUniqueTags(p.Tags)
	if err != nil {
		return core.WorkEntry{}, err
	}

	d, closer, err := open()
	if err != nil {
		return core.WorkEntry{}, err
	}
	defer closer()

	status := core.StatusNew
	if !p.NoSelect {
		status = core.StatusInProgress
	}
	entry, err := core.NewWorkEntry(core.NewWorkEntryParams{
		Title:          p.Title,
		Description:    p.Description,
		ScratchpadPath: p.ScratchpadPath,
		Status:         status,
	})
	if err != nil {
		return core.WorkEntry{}, err
	}
	if err := db.InsertWorkEntry(ctx, d, entry); err != nil {
		return core.WorkEntry{}, err
	}
	for _, name := range tags {
		if err := db.AddTagToWorkEntry(ctx, d, entry.ID, name); err != nil {
			return core.WorkEntry{}, err
		}
	}
	sort.Strings(tags) // mirror the storage layer's alphabetical order
	entry.Tags = tags

	if !p.NoSelect {
		if err := db.SelectWorkEntry(ctx, d, entry.ID); err != nil {
			return core.WorkEntry{}, err
		}
	}
	return entry, nil
}

// normalizeUniqueTags runs each raw tag through [core.NormalizeTagName]
// and drops duplicates after normalization, preserving first-seen
// order. Returns the first validation error; on success the slice is
// safe to hand straight to the db layer.
func normalizeUniqueTags(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, r := range raw {
		name, err := core.NormalizeTagName(r)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}

// ListWork returns every work entry, newest first. An empty database
// is not an error — the returned slice is simply empty.
func ListWork(ctx context.Context) ([]core.WorkEntry, error) {
	d, closer, err := open()
	if err != nil {
		return nil, err
	}
	defer closer()

	return db.ListWorkEntries(ctx, d)
}

// ShowWork returns the work entry with the given ID, or wraps
// [db.ErrWorkEntryNotFound] if none exists. An empty id falls back
// to the currently selected entry; if nothing is selected it
// returns [ErrNoTargetWorkEntry].
func ShowWork(ctx context.Context, id string) (core.WorkEntry, error) {
	d, closer, err := open()
	if err != nil {
		return core.WorkEntry{}, err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return core.WorkEntry{}, err
	}
	return db.GetWorkEntry(ctx, d, target)
}

// SelectWork sets the given entry as the current focus and returns
// the (full) entry it just selected, so the caller can confirm by
// printing it. Returns [db.ErrWorkEntryNotFound] if id doesn't match
// any row — checked up front so the user gets a clean error instead
// of a raw FK violation.
func SelectWork(ctx context.Context, id string) (core.WorkEntry, error) {
	d, closer, err := open()
	if err != nil {
		return core.WorkEntry{}, err
	}
	defer closer()

	entry, err := db.GetWorkEntry(ctx, d, id)
	if err != nil {
		return core.WorkEntry{}, err
	}
	if err := db.SelectWorkEntry(ctx, d, entry.ID); err != nil {
		return core.WorkEntry{}, err
	}
	return entry, nil
}

// ForgetSelectedWork clears the current selection. Safe to call when
// nothing is selected (silent no-op at the storage layer).
func ForgetSelectedWork(ctx context.Context) error {
	d, closer, err := open()
	if err != nil {
		return err
	}
	defer closer()

	return db.ForgetSelectedWorkEntry(ctx, d)
}

// GetSelectedWork returns the currently selected work entry (with
// tags populated), or wraps [db.ErrNoSelectedEntry] if no entry is
// selected. Unlike the id-defaulting use cases, this one does NOT
// remap the sentinel — the caller explicitly asked about selection
// state, so the lower-level message is the right one.
func GetSelectedWork(ctx context.Context) (core.WorkEntry, error) {
	d, closer, err := open()
	if err != nil {
		return core.WorkEntry{}, err
	}
	defer closer()

	return db.GetSelectedWorkEntry(ctx, d)
}

// TagWork attaches a tag to a work entry. The raw tag name is
// normalized via [core.NormalizeTagName] (lower-case, trim); the
// normalized form is returned alongside the resolved entry id so
// the caller can echo exactly what was mutated. Idempotent.
//
// An empty id falls back to the currently selected entry (returns
// [ErrNoTargetWorkEntry] if nothing is selected). Validates the
// entry exists up front so the user gets a clean
// [db.ErrWorkEntryNotFound] instead of a raw FK violation.
func TagWork(ctx context.Context, id, rawTag string) (resolvedID, tagName string, err error) {
	name, err := core.NormalizeTagName(rawTag)
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
	if err := db.AddTagToWorkEntry(ctx, d, target, name); err != nil {
		return "", "", err
	}
	return target, name, nil
}

// UntagWork removes a tag from a work entry. Returns the resolved
// id and normalized tag name so the caller can echo the mutation.
// Wraps [db.ErrTagNotOnEntry] if the entry does not have the tag,
// [db.ErrWorkEntryNotFound] if the entry itself is missing, and
// [ErrNoTargetWorkEntry] if no id was given and nothing is selected.
func UntagWork(ctx context.Context, id, rawTag string) (resolvedID, tagName string, err error) {
	name, err := core.NormalizeTagName(rawTag)
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
	if err := db.RemoveTagFromWorkEntry(ctx, d, target, name); err != nil {
		return "", "", err
	}
	return target, name, nil
}
