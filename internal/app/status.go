package app

import (
	"context"
	"errors"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
)

// MaxStatusActiveEntries caps how many active entries the status
// overview lists. The full count is still reported so the user knows
// the list was truncated; the cap just keeps the dashboard glanceable.
const MaxStatusActiveEntries = 10

// StatusOverview is the data behind `orbit status`: the currently
// selected entry (if any) plus the active entries (status new,
// in-progress, or paused), most recent first.
type StatusOverview struct {
	// Selected is the currently selected entry, or nil when nothing
	// is selected.
	Selected *core.WorkEntry
	// Active holds the active entries to display, capped at
	// [MaxStatusActiveEntries] and ordered most recent first.
	Active []core.WorkEntry
	// ActiveTotal is the total number of active entries before the
	// cap was applied, so callers can note any truncation.
	ActiveTotal int
}

// Status is the use case behind `orbit status`: gather a quick
// overview of current state. It reports the selected entry (or none)
// and the active entries — those whose status is [core.StatusNew],
// [core.StatusInProgress], or [core.StatusPaused] — newest first and
// capped at [MaxStatusActiveEntries].
//
// An empty database is not an error: the overview simply has no
// selection and no active entries.
func Status(ctx context.Context) (StatusOverview, error) {
	d, closer, err := open()
	if err != nil {
		return StatusOverview{}, err
	}
	defer closer()

	var overview StatusOverview

	selected, err := db.GetSelectedWorkEntry(ctx, d)
	switch {
	case errors.Is(err, db.ErrNoSelectedEntry):
		// No selection — leave overview.Selected nil.
	case err != nil:
		return StatusOverview{}, err
	default:
		overview.Selected = &selected
	}

	entries, err := db.ListWorkEntries(ctx, d)
	if err != nil {
		return StatusOverview{}, err
	}

	// ListWorkEntries already returns newest-first, so filtering in
	// place preserves the desired order.
	active := make([]core.WorkEntry, 0, len(entries))
	for _, e := range entries {
		if e.Status == core.StatusNew || e.Status == core.StatusInProgress || e.Status == core.StatusPaused {
			active = append(active, e)
		}
	}
	overview.ActiveTotal = len(active)
	if len(active) > MaxStatusActiveEntries {
		active = active[:MaxStatusActiveEntries]
	}
	overview.Active = active

	return overview, nil
}
