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

// TestDeleteWorkEntry_CascadesArtifacts is the acceptance test from the
// issue: deleting a work entry must take its artifacts (including note
// artifacts) with it via the schema-level ON DELETE CASCADE.
func TestDeleteWorkEntry_CascadesArtifacts(t *testing.T) {
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
	if err := AddArtifact(ctx, db, core.Artifact{WorkEntryID: entry.ID, Type: core.ArtifactNote, Value: "/n/a.md", CreatedAt: now}); err != nil {
		t.Fatalf("AddArtifact(note): %v", err)
	}

	if err := DeleteWorkEntry(ctx, db, entry.ID); err != nil {
		t.Fatalf("DeleteWorkEntry: %v", err)
	}

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM artifacts WHERE work_entry_id = ?`, entry.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count artifacts: %v", err)
	}
	if count != 0 {
		t.Errorf("artifacts rows after delete = %d, want 0 — cascade did not fire", count)
	}
}
