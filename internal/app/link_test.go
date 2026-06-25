package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
)

// seedSelectedEntry creates a work entry and selects it, returning its
// id. Used by the link tests that exercise the selected-entry default.
func seedSelectedEntry(t *testing.T, title string) string {
	t.Helper()
	entry, err := CreateWork(context.Background(), CreateWorkParams{Title: title})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}
	return entry.ID
}

func TestLinkArtifact_AddListRemove_EveryType(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := seedSelectedEntry(t, "all types")

	// One representative value per type. URL types need URL-shaped
	// values; path types are absolutized by the app layer.
	cases := []struct {
		typ core.ArtifactType
		val string
	}{
		{core.ArtifactBranch, "feature/x"},
		{core.ArtifactPR, "https://h/pr/1"},
		{core.ArtifactWorkItem, "https://ado/wi/2"},
		{core.ArtifactRepo, t.TempDir()},
		{core.ArtifactDir, t.TempDir()},
		{core.ArtifactFile, filepath.Join(t.TempDir(), "f.txt")},
		{core.ArtifactURL, "https://docs/page"},
		{core.ArtifactNote, filepath.Join(t.TempDir(), "note.md")},
		{core.ArtifactCustom, "anything goes"},
	}

	for _, c := range cases {
		// Empty id → selected entry.
		resolved, _, _, err := LinkArtifact(ctx, "", c.typ, c.val)
		if err != nil {
			t.Fatalf("LinkArtifact(%s): %v", c.typ, err)
		}
		if resolved != id {
			t.Errorf("LinkArtifact(%s) resolved %q, want selected %q", c.typ, resolved, id)
		}
	}

	_, artifacts, err := ListLinks(ctx, id)
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if len(artifacts) != len(cases) {
		t.Fatalf("linked %d artifacts, want %d", len(artifacts), len(cases))
	}

	// Every type can be removed.
	for _, c := range cases {
		if _, _, err := UnlinkArtifact(ctx, id, c.typ, c.val); err != nil {
			t.Errorf("UnlinkArtifact(%s): %v", c.typ, err)
		}
	}
	_, artifacts, err = ListLinks(ctx, id)
	if err != nil {
		t.Fatalf("ListLinks after remove: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("after removal %d artifacts remain, want 0", len(artifacts))
	}
}

func TestLinkArtifact_LocalPathStoredAbsolute(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := seedSelectedEntry(t, "abs path")

	// Run from a known cwd and link a relative path.
	cwd := t.TempDir()
	t.Chdir(cwd)

	_, stored, _, err := LinkArtifact(ctx, id, core.ArtifactDir, "subdir")
	if err != nil {
		t.Fatalf("LinkArtifact: %v", err)
	}
	want := filepath.Join(cwd, "subdir")
	if stored != want {
		t.Errorf("stored value = %q, want absolute %q", stored, want)
	}
}

func TestLinkArtifact_MissingPathIsWarningNotError(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := seedSelectedEntry(t, "missing path")

	missing := filepath.Join(t.TempDir(), "nope.txt")
	_, _, warning, err := LinkArtifact(ctx, id, core.ArtifactFile, missing)
	if err != nil {
		t.Fatalf("LinkArtifact should not error on missing path: %v", err)
	}
	if warning == "" {
		t.Error("expected a warning for a non-existent path, got none")
	}

	// An existing path produces no warning.
	existing := filepath.Join(t.TempDir(), "here.txt")
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	_, _, warning, err = LinkArtifact(ctx, id, core.ArtifactFile, existing)
	if err != nil {
		t.Fatalf("LinkArtifact: %v", err)
	}
	if warning != "" {
		t.Errorf("unexpected warning for existing path: %q", warning)
	}
}

func TestLinkArtifact_InvalidURLRejected(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := seedSelectedEntry(t, "bad url")

	if _, _, _, err := LinkArtifact(ctx, id, core.ArtifactURL, "not a url"); err == nil {
		t.Error("LinkArtifact with invalid URL = nil error, want error")
	}
}

// TestLinkArtifact_NoteStoredAbsolute confirms a note is just a
// local-path artifact: its path is absolutized and it lists back like
// any other artifact, with no date semantics.
func TestLinkArtifact_NoteStoredAbsolute(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := seedSelectedEntry(t, "note artifact")

	cwd := t.TempDir()
	t.Chdir(cwd)

	_, path, _, err := LinkArtifact(ctx, id, core.ArtifactNote, "notes/day.md")
	if err != nil {
		t.Fatalf("LinkArtifact(note): %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("note path %q is not absolute", path)
	}

	_, artifacts, err := ListLinks(ctx, id)
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].Type != core.ArtifactNote {
		t.Fatalf("artifacts = %+v, want one note artifact", artifacts)
	}
	if artifacts[0].Value != filepath.Join(cwd, "notes/day.md") {
		t.Errorf("note value = %q, want absolute path under cwd", artifacts[0].Value)
	}
}

func TestLink_NoTargetWhenNothingSelected(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()

	if _, _, _, err := LinkArtifact(ctx, "", core.ArtifactBranch, "x"); !errors.Is(err, ErrNoTargetWorkEntry) {
		t.Errorf("LinkArtifact err = %v, want ErrNoTargetWorkEntry", err)
	}
	if _, _, err := ListLinks(ctx, ""); !errors.Is(err, ErrNoTargetWorkEntry) {
		t.Errorf("ListLinks err = %v, want ErrNoTargetWorkEntry", err)
	}
}

func TestLinkArtifact_UnknownEntry(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()

	if _, _, _, err := LinkArtifact(ctx, "zzzzz", core.ArtifactBranch, "x"); !errors.Is(err, db.ErrWorkEntryNotFound) {
		t.Errorf("err = %v, want ErrWorkEntryNotFound", err)
	}
}
