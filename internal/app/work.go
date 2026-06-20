package app

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

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
	Title       string
	Description string
	// PadPath is the user-supplied <name> for the pad. When non-empty
	// it is resolved per [ResolvePadPath] and the resulting directory
	// is provisioned on disk before the entry is inserted; the
	// absolute path is what gets stored on the entry.
	PadPath string
	// NoDock, when true, forces the pad to be created relative to the
	// current working directory even when a dock root is configured.
	// Has no effect when PadPath is empty or absolute.
	NoDock bool
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
//
// If the pad directory already existed on disk, the entry is still
// fully created and persisted, and CreateWork returns the entry
// together with [ErrPadAlreadyExisted]. That sentinel signals
// "success, but worth telling the user" — callers MUST check it
// before treating err as a failure, e.g.:
//
//	entry, err := app.CreateWork(ctx, p)
//	if errors.Is(err, app.ErrPadAlreadyExisted) {
//		// inform the user, then drop the sentinel
//		err = nil
//	}
//	if err != nil { return err }
func CreateWork(ctx context.Context, p CreateWorkParams) (core.WorkEntry, error) {
	tags, err := normalizeUniqueTags(p.Tags)
	if err != nil {
		return core.WorkEntry{}, err
	}

	// Resolve and provision the pad upfront so a failure here aborts
	// before any DB writes. The "already existed" case is a warning,
	// not an error — it lets the user point a new entry at a folder
	// they already had. The absolute path is what we store on the
	// entry so `work show` and downstream tooling see a stable value.
	padAbs := ""
	padExisted := false
	if p.PadPath != "" {
		abs, err := ResolvePadPath(ctx, p.PadPath, p.NoDock)
		if err != nil {
			return core.WorkEntry{}, err
		}
		if perr := ProvisionPad(abs); perr != nil {
			if !errors.Is(perr, ErrPadAlreadyExisted) {
				return core.WorkEntry{}, perr
			}
			padExisted = true
		}
		padAbs = abs
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
		Title:       p.Title,
		Description: p.Description,
		PadPath:     padAbs,
		Status:      status,
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
	if padExisted {
		return entry, ErrPadAlreadyExisted
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

// ListWork returns work entries, newest first. When filterTags is
// non-empty the result is narrowed to entries carrying every one of
// the (normalized) tags — repeatable AND semantics, matching
// `orbit work list --tag a --tag b`. An empty database, or a filter
// that matches nothing, is not an error — the returned slice is simply
// empty. Filter tags are normalized via [core.NormalizeTagName] so the
// match is case-insensitive and consistent with how tags are stored.
func ListWork(ctx context.Context, filterTags []string) ([]core.WorkEntry, error) {
	want, err := normalizeUniqueTags(filterTags)
	if err != nil {
		return nil, err
	}

	d, closer, err := open()
	if err != nil {
		return nil, err
	}
	defer closer()

	entries, err := db.ListWorkEntries(ctx, d)
	if err != nil {
		return nil, err
	}
	if len(want) == 0 {
		return entries, nil
	}
	return filterEntriesByTags(entries, want), nil
}

// filterEntriesByTags keeps only the entries whose Tags contain every
// name in want (AND semantics). want is assumed already normalized;
// entry tags are stored normalized, so a direct set membership test is
// correct.
func filterEntriesByTags(entries []core.WorkEntry, want []string) []core.WorkEntry {
	out := make([]core.WorkEntry, 0, len(entries))
	for _, e := range entries {
		has := make(map[string]struct{}, len(e.Tags))
		for _, t := range e.Tags {
			has[t] = struct{}{}
		}
		match := true
		for _, w := range want {
			if _, ok := has[w]; !ok {
				match = false
				break
			}
		}
		if match {
			out = append(out, e)
		}
	}
	return out
}

// ListTags returns the plain (non-reserved) tag names on a work entry,
// sorted. Reserved `project:*` / `owner:*` tags are excluded — they
// have dedicated `work project list` / `work owner list` views — so
// `work tag list` shows exactly the free-form labels. An entry with no
// plain tags yields a nil slice, not an error.
//
// An empty id falls back to the currently selected entry; wraps
// [db.ErrWorkEntryNotFound] / [ErrNoTargetWorkEntry] as the other
// per-entry use cases do.
func ListTags(ctx context.Context, id string) (resolvedID string, tags []string, err error) {
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
	_, _, plain := core.PartitionReservedTags(entry.Tags)
	return target, plain, nil
}

// ListAllTags returns the global tag vocabulary with per-tag work-entry
// counts, alphabetical. Reserved `project:*` / `owner:*` tags are
// omitted so they aren't surfaced twice — they have their own
// project/owner views. An empty vocabulary returns a nil slice, not an
// error.
func ListAllTags(ctx context.Context) ([]core.TagCount, error) {
	d, closer, err := open()
	if err != nil {
		return nil, err
	}
	defer closer()

	all, err := db.ListAllTags(ctx, d)
	if err != nil {
		return nil, err
	}
	var out []core.TagCount
	for _, tc := range all {
		if core.IsReservedTag(tc.Name) {
			continue
		}
		out = append(out, tc)
	}
	return out, nil
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

// DeleteWork removes the work entry with the given id and returns
// the entry as it existed immediately before the delete — so the
// caller can echo the title and the (now-orphaned) pad path.
// Wraps [db.ErrWorkEntryNotFound] if id doesn't match any row.
//
// Load and delete run in two SQL statements (not a transaction),
// which leaves a tiny window in which a concurrent delete could
// win between the GET and the DELETE. In that case our DELETE
// affects zero rows and the function returns ErrWorkEntryNotFound
// — same outcome as if the id had been wrong all along. Orbit is
// a single-user, no-daemon tool so this race is essentially never
// taken; revisit if/when the app package grows a tx helper.
//
// The pad folder on disk is intentionally NOT touched here — that
// behavior belongs to `orbit work delete --purge` and lives in
// the CLI layer (separate commit). Schema-level cascades take
// care of the tag-join rows and the selected-entry pointer; the
// shared `tags` vocabulary is preserved.
func DeleteWork(ctx context.Context, id string) (core.WorkEntry, error) {
	d, closer, err := open()
	if err != nil {
		return core.WorkEntry{}, err
	}
	defer closer()

	entry, err := db.GetWorkEntry(ctx, d, id)
	if err != nil {
		return core.WorkEntry{}, err
	}
	if err := db.DeleteWorkEntry(ctx, d, id); err != nil {
		return core.WorkEntry{}, err
	}
	return entry, nil
}

// SetPad resolves rawPath (respecting the dock root unless noDock
// is true), provisions the directory on disk if needed, and writes
// the absolute path onto the entry. Returns the fully-updated
// entry.
//
// If the pad directory already existed on disk, the update is
// still applied and SetPad returns the entry together with
// [ErrPadAlreadyExisted] — same success-with-warning convention as
// [CreateWork]. Callers MUST check the sentinel before treating
// err as a failure.
//
// An empty rawPath clears the pad column. The directory on disk is
// intentionally NOT touched in either direction — disk removal
// belongs to `orbit work delete --purge` semantics.
//
// An empty id falls back to the currently selected entry; if
// nothing is selected, returns [ErrNoTargetWorkEntry]. Wraps
// [db.ErrWorkEntryNotFound] if the id doesn't match any row.
func SetPad(ctx context.Context, id, rawPath string, noDock bool) (core.WorkEntry, error) {
	d, closer, err := open()
	if err != nil {
		return core.WorkEntry{}, err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return core.WorkEntry{}, err
	}

	padAbs := ""
	padExisted := false
	if rawPath != "" {
		abs, err := ResolvePadPath(ctx, rawPath, noDock)
		if err != nil {
			return core.WorkEntry{}, err
		}
		if perr := ProvisionPad(abs); perr != nil {
			if !errors.Is(perr, ErrPadAlreadyExisted) {
				return core.WorkEntry{}, perr
			}
			padExisted = true
		}
		padAbs = abs
	}

	if err := db.UpdateWorkEntryPad(ctx, d, target, padAbs, time.Now().UTC()); err != nil {
		return core.WorkEntry{}, err
	}
	updated, err := db.GetWorkEntry(ctx, d, target)
	if err != nil {
		return core.WorkEntry{}, err
	}
	if padExisted {
		return updated, ErrPadAlreadyExisted
	}
	return updated, nil
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
