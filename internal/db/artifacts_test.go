package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lydietoure/orbit/internal/core"
)

// TestAddAndListArtifacts checks artifacts persist and read back oldest-first.
func TestAddAndListArtifacts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "linkable"})
	if err := InsertWorkEntry(ctx, db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	now := time.Now().UTC()
	artifacts := []core.Artifact{
		{WorkEntryID: entry.ID, Type: core.ArtifactBranch, Value: "feature/x", CreatedAt: now},
		{WorkEntryID: entry.ID, Type: core.ArtifactPR, Value: "https://h/pr/1", CreatedAt: now.Add(time.Second)},
	}
	for _, a := range artifacts {
		if err := AddArtifact(ctx, db, a); err != nil {
			t.Fatalf("AddArtifact %s: %v", a.Type, err)
		}
	}

	got, err := ListArtifactsForWorkEntry(ctx, db, entry.ID)
	if err != nil {
		t.Fatalf("ListArtifactsForWorkEntry: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// Oldest-first ordering.
	if got[0].Type != core.ArtifactBranch || got[1].Type != core.ArtifactPR {
		t.Errorf("order = [%s %s], want [branch pr]", got[0].Type, got[1].Type)
	}
	if got[0].Value != "feature/x" {
		t.Errorf("value = %q, want feature/x", got[0].Value)
	}
}

func TestAddArtifact_Idempotent(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "dup"})
	if err := InsertWorkEntry(ctx, db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	a := core.Artifact{WorkEntryID: entry.ID, Type: core.ArtifactBranch, Value: "main", CreatedAt: time.Now().UTC()}
	for i := 0; i < 2; i++ {
		if err := AddArtifact(ctx, db, a); err != nil {
			t.Fatalf("AddArtifact #%d: %v", i, err)
		}
	}
	got, err := ListArtifactsForWorkEntry(ctx, db, entry.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1 (re-link should be a no-op)", len(got))
	}
}

func TestRemoveArtifact(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "removable"})
	if err := InsertWorkEntry(ctx, db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	a := core.Artifact{WorkEntryID: entry.ID, Type: core.ArtifactBranch, Value: "main", CreatedAt: time.Now().UTC()}
	if err := AddArtifact(ctx, db, a); err != nil {
		t.Fatalf("AddArtifact: %v", err)
	}

	if err := RemoveArtifact(ctx, db, entry.ID, core.ArtifactBranch, "main"); err != nil {
		t.Fatalf("RemoveArtifact: %v", err)
	}
	// Second removal reports not-on-entry.
	err := RemoveArtifact(ctx, db, entry.ID, core.ArtifactBranch, "main")
	if !errors.Is(err, ErrArtifactNotOnEntry) {
		t.Errorf("err = %v, want ErrArtifactNotOnEntry", err)
	}
}

func TestAddAndListNotes(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "noted"})
	if err := InsertWorkEntry(ctx, db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	now := time.Now().UTC()
	notes := []core.Note{
		{WorkEntryID: entry.ID, Path: "/n/older.md", Date: "2026-06-01", CreatedAt: now},
		{WorkEntryID: entry.ID, Path: "/n/newer.md", Date: "2026-06-20", CreatedAt: now},
	}
	for _, n := range notes {
		if err := AddNote(ctx, db, n); err != nil {
			t.Fatalf("AddNote %s: %v", n.Path, err)
		}
	}

	got, err := ListNotesForWorkEntry(ctx, db, entry.ID)
	if err != nil {
		t.Fatalf("ListNotesForWorkEntry: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// Newest date first.
	if got[0].Date != "2026-06-20" {
		t.Errorf("first note date = %q, want 2026-06-20", got[0].Date)
	}
}

func TestAddNote_SamePathDifferentDateAreDistinct(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "dates"})
	if err := InsertWorkEntry(ctx, db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	now := time.Now().UTC()
	// Same path, two dates → two notes. Re-adding the first is a no-op.
	for _, n := range []core.Note{
		{WorkEntryID: entry.ID, Path: "/n/a.md", Date: "2026-06-01", CreatedAt: now},
		{WorkEntryID: entry.ID, Path: "/n/a.md", Date: "2026-06-01", CreatedAt: now},
		{WorkEntryID: entry.ID, Path: "/n/a.md", Date: "2026-06-02", CreatedAt: now},
	} {
		if err := AddNote(ctx, db, n); err != nil {
			t.Fatalf("AddNote: %v", err)
		}
	}
	got, err := ListNotesForWorkEntry(ctx, db, entry.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestRemoveNote_RemovesAllDatesForPath(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "rmnote"})
	if err := InsertWorkEntry(ctx, db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	now := time.Now().UTC()
	for _, d := range []string{"2026-06-01", "2026-06-02"} {
		if err := AddNote(ctx, db, core.Note{WorkEntryID: entry.ID, Path: "/n/a.md", Date: d, CreatedAt: now}); err != nil {
			t.Fatalf("AddNote: %v", err)
		}
	}
	if err := RemoveNote(ctx, db, entry.ID, "/n/a.md"); err != nil {
		t.Fatalf("RemoveNote: %v", err)
	}
	got, err := ListNotesForWorkEntry(ctx, db, entry.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0 (all dates for the path removed)", len(got))
	}
	if err := RemoveNote(ctx, db, entry.ID, "/n/a.md"); !errors.Is(err, ErrNoteNotOnEntry) {
		t.Errorf("err = %v, want ErrNoteNotOnEntry", err)
	}
}

// TestDeleteWorkEntry_CascadesArtifactsAndNotes is the acceptance test
// from the issue: deleting a work entry must take its artifacts and
// notes with it via the schema-level ON DELETE CASCADE.
func TestDeleteWorkEntry_CascadesArtifactsAndNotes(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "doomed"})
	if err := InsertWorkEntry(ctx, db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	now := time.Now().UTC()
	if err := AddArtifact(ctx, db, core.Artifact{WorkEntryID: entry.ID, Type: core.ArtifactBranch, Value: "main", CreatedAt: now}); err != nil {
		t.Fatalf("AddArtifact: %v", err)
	}
	if err := AddNote(ctx, db, core.Note{WorkEntryID: entry.ID, Path: "/n/a.md", Date: "2026-06-20", CreatedAt: now}); err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	if err := DeleteWorkEntry(ctx, db, entry.ID); err != nil {
		t.Fatalf("DeleteWorkEntry: %v", err)
	}

	for _, tbl := range []string{"artifacts", "notes"} {
		var count int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM `+tbl+` WHERE work_entry_id = ?`, entry.ID,
		).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		if count != 0 {
			t.Errorf("%s rows after delete = %d, want 0 — cascade did not fire", tbl, count)
		}
	}
}
