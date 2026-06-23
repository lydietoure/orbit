package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	_, artifacts, _, err := ListLinks(ctx, id)
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
	_, artifacts, _, err = ListLinks(ctx, id)
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

func TestLinkNote_RecordsDate(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := seedSelectedEntry(t, "note date")

	_, path, date, _, err := LinkNote(ctx, id, "notes/day.md", "2026-06-20")
	if err != nil {
		t.Fatalf("LinkNote: %v", err)
	}
	if date != "2026-06-20" {
		t.Errorf("date = %q, want 2026-06-20", date)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("note path %q is not absolute", path)
	}

	_, _, notes, err := ListLinks(ctx, id)
	if err != nil {
		t.Fatalf("ListLinks: %v", err)
	}
	if len(notes) != 1 || notes[0].Date != "2026-06-20" {
		t.Fatalf("notes = %+v, want one dated 2026-06-20", notes)
	}
}

func TestLinkNote_DefaultsDateToToday(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := seedSelectedEntry(t, "note today")

	_, _, date, _, err := LinkNote(ctx, id, "notes/x.md", "")
	if err != nil {
		t.Fatalf("LinkNote: %v", err)
	}
	want := time.Now().UTC().Format(core.NoteDateLayout)
	if date != want {
		t.Errorf("date = %q, want today %q", date, want)
	}
}

func TestLink_NoTargetWhenNothingSelected(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()

	if _, _, _, err := LinkArtifact(ctx, "", core.ArtifactBranch, "x"); !errors.Is(err, ErrNoTargetWorkEntry) {
		t.Errorf("LinkArtifact err = %v, want ErrNoTargetWorkEntry", err)
	}
	if _, _, _, err := ListLinks(ctx, ""); !errors.Is(err, ErrNoTargetWorkEntry) {
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
