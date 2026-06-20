package app

import (
	"context"
	"testing"

	"github.com/lydietoure/orbit/internal/core"
)

// TestStatus_EmptyDatabase confirms the graceful empty state: no
// selection and no active entries on a freshly initialized home.
func TestStatus_EmptyDatabase(t *testing.T) {
	setupInitializedHome(t)

	overview, err := Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if overview.Selected != nil {
		t.Errorf("Selected = %+v, want nil", overview.Selected)
	}
	if len(overview.Active) != 0 {
		t.Errorf("Active = %v, want empty", overview.Active)
	}
	if overview.ActiveTotal != 0 {
		t.Errorf("ActiveTotal = %d, want 0", overview.ActiveTotal)
	}
}

// TestStatus_ReportsSelectedAndActive checks the populated path: the
// selected entry is reported, and only new/in-progress entries are
// counted as active (completed/abandoned are excluded).
func TestStatus_ReportsSelectedAndActive(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()

	// Auto-selected and promoted to in-progress.
	selected, err := CreateWork(ctx, CreateWorkParams{Title: "selected entry"})
	if err != nil {
		t.Fatalf("CreateWork selected: %v", err)
	}
	// Another active entry (status new via NoSelect).
	if _, err := CreateWork(ctx, CreateWorkParams{Title: "queued entry", NoSelect: true}); err != nil {
		t.Fatalf("CreateWork queued: %v", err)
	}

	overview, err := Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if overview.Selected == nil {
		t.Fatal("Selected = nil, want the selected entry")
	}
	if overview.Selected.ID != selected.ID {
		t.Errorf("Selected.ID = %q, want %q", overview.Selected.ID, selected.ID)
	}
	if overview.ActiveTotal != 2 {
		t.Errorf("ActiveTotal = %d, want 2", overview.ActiveTotal)
	}
	if len(overview.Active) != 2 {
		t.Fatalf("len(Active) = %d, want 2", len(overview.Active))
	}
	for _, e := range overview.Active {
		if e.Status != core.StatusNew && e.Status != core.StatusInProgress {
			t.Errorf("active entry %s has non-active status %q", e.ID, e.Status)
		}
	}
}

// TestStatus_CapsActiveEntries confirms that the displayed list is
// capped at MaxStatusActiveEntries while ActiveTotal reflects the full
// count, so callers can report truncation.
func TestStatus_CapsActiveEntries(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()

	const total = MaxStatusActiveEntries + 3
	for i := 0; i < total; i++ {
		title := "entry-" + string(rune('a'+i))
		if _, err := CreateWork(ctx, CreateWorkParams{Title: title, NoSelect: true}); err != nil {
			t.Fatalf("CreateWork %d: %v", i, err)
		}
	}

	overview, err := Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if overview.ActiveTotal != total {
		t.Errorf("ActiveTotal = %d, want %d", overview.ActiveTotal, total)
	}
	if len(overview.Active) != MaxStatusActiveEntries {
		t.Errorf("len(Active) = %d, want %d", len(overview.Active), MaxStatusActiveEntries)
	}
}
