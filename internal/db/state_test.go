package db

import (
	"context"
	"errors"
	"testing"

	"github.com/lydietoure/orbit/internal/core"
)

func TestGetSelectedWorkEntry_None(t *testing.T) {
	db := newTestDB(t)

	_, err := GetSelectedWorkEntry(context.Background(), db)
	if err == nil {
		t.Fatal("expected error on empty selection, got nil")
	}
	if !errors.Is(err, ErrNoSelectedEntry) {
		t.Errorf("err = %v, want ErrNoSelectedEntry", err)
	}
}

func TestSelectWorkEntry_PersistsAndGetReturnsIt(t *testing.T) {
	db := newTestDB(t)
	want := makeValidEntry(t, core.NewWorkEntryParams{Title: "selected one"})
	if err := InsertWorkEntry(context.Background(), db, want); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	if err := SelectWorkEntry(context.Background(), db, want.ID); err != nil {
		t.Fatalf("SelectWorkEntry: %v", err)
	}

	got, err := GetSelectedWorkEntry(context.Background(), db)
	if err != nil {
		t.Fatalf("GetSelectedWorkEntry: %v", err)
	}
	if got.ID != want.ID || got.Title != want.Title {
		t.Errorf("selected entry = %+v, want id=%s title=%q", got, want.ID, want.Title)
	}
}

func TestSelectWorkEntry_OverwritesPrevious(t *testing.T) {
	db := newTestDB(t)
	first := makeValidEntry(t, core.NewWorkEntryParams{Title: "first"})
	second := makeValidEntry(t, core.NewWorkEntryParams{Title: "second"})
	for _, e := range []core.WorkEntry{first, second} {
		if err := InsertWorkEntry(context.Background(), db, e); err != nil {
			t.Fatalf("InsertWorkEntry %s: %v", e.ID, err)
		}
	}

	if err := SelectWorkEntry(context.Background(), db, first.ID); err != nil {
		t.Fatalf("Select first: %v", err)
	}
	if err := SelectWorkEntry(context.Background(), db, second.ID); err != nil {
		t.Fatalf("Select second: %v", err)
	}

	got, err := GetSelectedWorkEntry(context.Background(), db)
	if err != nil {
		t.Fatalf("GetSelectedWorkEntry: %v", err)
	}
	if got.ID != second.ID {
		t.Errorf("selected id = %s, want %s (second overwrote first)", got.ID, second.ID)
	}
}

func TestSelectWorkEntry_RejectsMissingID(t *testing.T) {
	db := newTestDB(t)

	// No entries exist; FK on state.selected_work_entry_id must reject.
	if err := SelectWorkEntry(context.Background(), db, "ghost"); err == nil {
		t.Fatal("expected FK error for missing id, got nil")
	}
}

func TestForgetSelectedWorkEntry_ClearsSelection(t *testing.T) {
	db := newTestDB(t)
	e := makeValidEntry(t, core.NewWorkEntryParams{Title: "pick me"})
	if err := InsertWorkEntry(context.Background(), db, e); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	if err := SelectWorkEntry(context.Background(), db, e.ID); err != nil {
		t.Fatalf("SelectWorkEntry: %v", err)
	}

	if err := ForgetSelectedWorkEntry(context.Background(), db); err != nil {
		t.Fatalf("ForgetSelectedWorkEntry: %v", err)
	}

	_, err := GetSelectedWorkEntry(context.Background(), db)
	if !errors.Is(err, ErrNoSelectedEntry) {
		t.Errorf("after Forget, Get err = %v, want ErrNoSelectedEntry", err)
	}
}

func TestForgetSelectedWorkEntry_NoOpWhenNoneSelected(t *testing.T) {
	db := newTestDB(t)

	// Fresh DB has no selection; Forget should silently succeed.
	if err := ForgetSelectedWorkEntry(context.Background(), db); err != nil {
		t.Errorf("ForgetSelectedWorkEntry on fresh DB: %v", err)
	}
}
