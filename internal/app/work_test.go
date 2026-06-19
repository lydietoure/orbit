package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
