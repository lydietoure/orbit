package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lydietoure/orbit/internal/db"
)

// TestCreateWork_ProvisionsPadAndStoresAbsolutePath is the end-to-end
// check for `orbit work new <title> -p <name>`: with a dock root set,
// CreateWork must (a) join name under the dock root, (b) mkdir the
// resulting directory, and (c) store the absolute path on the entry.
func TestCreateWork_ProvisionsPadAndStoresAbsolutePath(t *testing.T) {
	setupInitializedHome(t)
	dock := t.TempDir()
	t.Setenv(DockEnv, dock)

	entry, err := CreateWork(context.Background(), CreateWorkParams{
		Title:    "rename test",
		PadPath:  "pad-experiments",
		NoSelect: true, // avoid touching the select state for this test
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	wantPad := filepath.Join(dock, "pad-experiments")
	if entry.PadPath != wantPad {
		t.Errorf("entry.PadPath = %q, want %q", entry.PadPath, wantPad)
	}
	info, err := os.Stat(wantPad)
	if err != nil {
		t.Fatalf("expected pad dir on disk: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("pad path is not a directory: %v", info.Mode())
	}
}

// TestCreateWork_NoDockProvisionsUnderCWD covers --no-dock: with a
// dock root configured but NoDock=true, the pad is created under the
// current working directory.
func TestCreateWork_NoDockProvisionsUnderCWD(t *testing.T) {
	setupInitializedHome(t)
	dock := t.TempDir()
	t.Setenv(DockEnv, dock)

	cwd := t.TempDir()
	t.Chdir(cwd)

	entry, err := CreateWork(context.Background(), CreateWorkParams{
		Title:    "no-dock test",
		PadPath:  "in-cwd-pad",
		NoDock:   true,
		NoSelect: true,
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	wantPad := filepath.Join(cwd, "in-cwd-pad")
	if entry.PadPath != wantPad {
		t.Errorf("entry.PadPath = %q, want %q", entry.PadPath, wantPad)
	}
	if _, err := os.Stat(wantPad); err != nil {
		t.Errorf("expected pad dir on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dock, "in-cwd-pad")); err == nil {
		t.Errorf("pad unexpectedly created under dock root despite NoDock=true")
	}
}

// TestCreateWork_PadAlreadyExistedSucceeds confirms that pointing a
// new entry at a pre-existing folder is not an error: the entry is
// still created and the path is stored as-is.
func TestCreateWork_PadAlreadyExistedSucceeds(t *testing.T) {
	setupInitializedHome(t)
	dock := t.TempDir()
	t.Setenv(DockEnv, dock)

	preexisting := filepath.Join(dock, "already-here")
	if err := os.MkdirAll(preexisting, 0o755); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	entry, err := CreateWork(context.Background(), CreateWorkParams{
		Title:    "preexisting test",
		PadPath:  "already-here",
		NoSelect: true,
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}
	if entry.PadPath != preexisting {
		t.Errorf("entry.PadPath = %q, want %q", entry.PadPath, preexisting)
	}
}

// TestCreateWork_NoPadFlagLeavesPathEmpty confirms the default case:
// no -p flag means no pad provisioning and an empty PadPath on the
// stored entry.
func TestCreateWork_NoPadFlagLeavesPathEmpty(t *testing.T) {
	setupInitializedHome(t)
	dock := t.TempDir()
	t.Setenv(DockEnv, dock)

	entry, err := CreateWork(context.Background(), CreateWorkParams{
		Title:    "no pad",
		NoSelect: true,
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}
	if entry.PadPath != "" {
		t.Errorf("entry.PadPath = %q, want empty", entry.PadPath)
	}
	// The dock dir should still be empty — we provisioned nothing.
	entries, err := os.ReadDir(dock)
	if err != nil {
		t.Fatalf("read dock: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("dock has %d entries, want 0", len(entries))
	}
}

// TestDeleteWork_ReturnsPreDeleteEntryAndRemovesRow is the happy
// path: DeleteWork hands back the entry snapshot the user can echo
// (title, pad path, etc.) AND the underlying row is gone so a
// follow-up ShowWork wraps ErrWorkEntryNotFound.
func TestDeleteWork_ReturnsPreDeleteEntryAndRemovesRow(t *testing.T) {
	setupInitializedHome(t)
	dock := t.TempDir()
	t.Setenv(DockEnv, dock)

	created, err := CreateWork(context.Background(), CreateWorkParams{
		Title:    "doomed entry",
		PadPath:  "doomed-pad",
		NoSelect: true,
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	deleted, err := DeleteWork(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("DeleteWork: %v", err)
	}
	if deleted.ID != created.ID {
		t.Errorf("deleted.ID = %q, want %q", deleted.ID, created.ID)
	}
	if deleted.Title != created.Title {
		t.Errorf("deleted.Title = %q, want %q", deleted.Title, created.Title)
	}
	wantPad := filepath.Join(dock, "doomed-pad")
	if deleted.PadPath != wantPad {
		t.Errorf("deleted.PadPath = %q, want %q (caller needs this to print the orphaned-pad note)",
			deleted.PadPath, wantPad)
	}

	// Pad folder on disk must NOT be touched — that is --purge's job.
	if _, err := os.Stat(wantPad); err != nil {
		t.Errorf("pad folder removed by DeleteWork (must be left alone in M0 bare delete): %v", err)
	}

	// Row really gone.
	if _, err := ShowWork(context.Background(), created.ID); !errors.Is(err, db.ErrWorkEntryNotFound) {
		t.Errorf("post-delete ShowWork err = %v, want db.ErrWorkEntryNotFound", err)
	}
}

// TestDeleteWork_UnknownIDReturnsNotFound covers the typo case: an
// id that never existed must wrap db.ErrWorkEntryNotFound so the
// CLI can render the same clean message it does for `work show`.
func TestDeleteWork_UnknownIDReturnsNotFound(t *testing.T) {
	setupInitializedHome(t)

	_, err := DeleteWork(context.Background(), "ghost")
	if !errors.Is(err, db.ErrWorkEntryNotFound) {
		t.Errorf("err = %v, want db.ErrWorkEntryNotFound", err)
	}
}

// TestDeleteWork_ClearsSelectionIfSelected is the user-visible
// consequence of the state.selected_work_entry_id cascade: after
// deleting the currently selected entry, GetSelectedWork must
// report ErrNoSelectedEntry instead of pointing into the void.
func TestDeleteWork_ClearsSelectionIfSelected(t *testing.T) {
	setupInitializedHome(t)

	// Auto-select on creation (the default) puts this entry in the
	// state singleton; deleting it should clear that pointer.
	created, err := CreateWork(context.Background(), CreateWorkParams{
		Title: "selected and doomed",
	})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	if _, err := DeleteWork(context.Background(), created.ID); err != nil {
		t.Fatalf("DeleteWork: %v", err)
	}

	_, err = GetSelectedWork(context.Background())
	if !errors.Is(err, db.ErrNoSelectedEntry) {
		t.Errorf("post-delete GetSelectedWork err = %v, want db.ErrNoSelectedEntry "+
			"(schema cascade should have cleared the selection pointer)", err)
	}
}

// TestDeleteWork_NotInitialized confirms the use case fails clean
// when orbit hasn't been initialized — same contract as the other
// app functions.
func TestDeleteWork_NotInitialized(t *testing.T) {
	// Point ORBIT_HOME at an empty tempdir; do NOT seed a DB.
	home := t.TempDir()
	t.Setenv("ORBIT_HOME", home)
	t.Setenv(DockEnv, "")

	_, err := DeleteWork(context.Background(), "whatever")
	if !errors.Is(err, ErrNotInitialized) {
		t.Errorf("err = %v, want ErrNotInitialized", err)
	}
}
