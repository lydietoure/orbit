package app

import (
	"context"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
)

// CreateWorkParams is the input to [CreateWork]. Mirrors the cli flag
// set; the use case doesn't care where the values came from.
type CreateWorkParams struct {
	Title          string
	Description    string
	ScratchpadPath string
}

// CreateWork is the use case behind `orbit work new`: build a
// validated [core.WorkEntry] from the params and persist it.
func CreateWork(ctx context.Context, p CreateWorkParams) (core.WorkEntry, error) {
	d, closer, err := open()
	if err != nil {
		return core.WorkEntry{}, err
	}
	defer closer()

	entry, err := core.NewWorkEntry(core.NewWorkEntryParams{
		Title:          p.Title,
		Description:    p.Description,
		ScratchpadPath: p.ScratchpadPath,
	})
	if err != nil {
		return core.WorkEntry{}, err
	}
	if err := db.InsertWorkEntry(ctx, d, entry); err != nil {
		return core.WorkEntry{}, err
	}
	return entry, nil
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
