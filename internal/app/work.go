package app

import (
	"context"
	"sort"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
)

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
// [db.ErrWorkEntryNotFound] if none exists.
func ShowWork(ctx context.Context, id string) (core.WorkEntry, error) {
	d, closer, err := open()
	if err != nil {
		return core.WorkEntry{}, err
	}
	defer closer()

	return db.GetWorkEntry(ctx, d, id)
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

// TagWork attaches a tag to the given work entry. The raw tag name
// is normalized via [core.NormalizeTagName] (lower-case, trim); the
// normalized form is returned so the caller can echo what was
// actually stored. Idempotent: re-tagging is a no-op.
//
// Validates the entry exists up front so the user gets a clean
// [db.ErrWorkEntryNotFound] instead of a raw FK violation.
func TagWork(ctx context.Context, id, rawTag string) (string, error) {
	name, err := core.NormalizeTagName(rawTag)
	if err != nil {
		return "", err
	}

	d, closer, err := open()
	if err != nil {
		return "", err
	}
	defer closer()

	if _, err := db.GetWorkEntry(ctx, d, id); err != nil {
		return "", err
	}
	if err := db.AddTagToWorkEntry(ctx, d, id, name); err != nil {
		return "", err
	}
	return name, nil
}

// UntagWork removes a tag from the given work entry. The raw tag
// name is normalized for lookup; the normalized form is returned for
// echo. Wraps [db.ErrTagNotOnEntry] if the entry does not have the
// tag, and [db.ErrWorkEntryNotFound] if the entry itself is missing.
func UntagWork(ctx context.Context, id, rawTag string) (string, error) {
	name, err := core.NormalizeTagName(rawTag)
	if err != nil {
		return "", err
	}

	d, closer, err := open()
	if err != nil {
		return "", err
	}
	defer closer()

	if _, err := db.GetWorkEntry(ctx, d, id); err != nil {
		return "", err
	}
	if err := db.RemoveTagFromWorkEntry(ctx, d, id, name); err != nil {
		return "", err
	}
	return name, nil
}
