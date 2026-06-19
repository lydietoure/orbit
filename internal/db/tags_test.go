package db

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lydietoure/orbit/internal/core"
)

func TestEnsureTag_Idempotent(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	first, err := EnsureTag(ctx, db, "caching")
	if err != nil {
		t.Fatalf("EnsureTag first: %v", err)
	}
	second, err := EnsureTag(ctx, db, "caching")
	if err != nil {
		t.Fatalf("EnsureTag second: %v", err)
	}
	if first != second {
		t.Errorf("EnsureTag returned different ids: %d vs %d", first, second)
	}
}

func TestAddTagToWorkEntry_HappyPathAndIdempotent(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	e := makeValidEntry(t, core.NewWorkEntryParams{Title: "with tags"})
	if err := InsertWorkEntry(ctx, db, e); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	if err := AddTagToWorkEntry(ctx, db, e.ID, "caching"); err != nil {
		t.Fatalf("AddTagToWorkEntry: %v", err)
	}
	// Re-adding the same tag must not error and must not duplicate.
	if err := AddTagToWorkEntry(ctx, db, e.ID, "caching"); err != nil {
		t.Fatalf("AddTagToWorkEntry (idempotent): %v", err)
	}
	if err := AddTagToWorkEntry(ctx, db, e.ID, "perf"); err != nil {
		t.Fatalf("AddTagToWorkEntry second tag: %v", err)
	}

	tags, err := ListTagsForWorkEntry(ctx, db, e.ID)
	if err != nil {
		t.Fatalf("ListTagsForWorkEntry: %v", err)
	}
	want := []string{"caching", "perf"} // alphabetical
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %v, want %v", tags, want)
	}
}

func TestAddTagToWorkEntry_RejectsMissingEntry(t *testing.T) {
	db := newTestDB(t)
	if err := AddTagToWorkEntry(context.Background(), db, "ghost", "caching"); err == nil {
		t.Fatal("expected FK error for missing work entry, got nil")
	}
}

func TestRemoveTagFromWorkEntry_HappyPath(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	e := makeValidEntry(t, core.NewWorkEntryParams{Title: "to untag"})
	if err := InsertWorkEntry(ctx, db, e); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	if err := AddTagToWorkEntry(ctx, db, e.ID, "caching"); err != nil {
		t.Fatalf("AddTagToWorkEntry: %v", err)
	}

	if err := RemoveTagFromWorkEntry(ctx, db, e.ID, "caching"); err != nil {
		t.Fatalf("RemoveTagFromWorkEntry: %v", err)
	}
	tags, err := ListTagsForWorkEntry(ctx, db, e.ID)
	if err != nil {
		t.Fatalf("ListTagsForWorkEntry: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("tags after remove = %v, want empty", tags)
	}
}

func TestRemoveTagFromWorkEntry_NotPresent(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	e := makeValidEntry(t, core.NewWorkEntryParams{Title: "no tags"})
	if err := InsertWorkEntry(ctx, db, e); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	err := RemoveTagFromWorkEntry(ctx, db, e.ID, "never-added")
	if err == nil {
		t.Fatal("expected ErrTagNotOnEntry, got nil")
	}
	if !errors.Is(err, ErrTagNotOnEntry) {
		t.Errorf("err = %v, want ErrTagNotOnEntry", err)
	}
}

func TestListTagsForWorkEntry_EmptyIsNil(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	e := makeValidEntry(t, core.NewWorkEntryParams{Title: "no tags"})
	if err := InsertWorkEntry(ctx, db, e); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	tags, err := ListTagsForWorkEntry(ctx, db, e.ID)
	if err != nil {
		t.Fatalf("ListTagsForWorkEntry: %v", err)
	}
	if tags != nil {
		t.Errorf("tags = %v, want nil", tags)
	}
}

func TestGetWorkEntry_PopulatesTags(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	e := makeValidEntry(t, core.NewWorkEntryParams{Title: "tagged"})
	if err := InsertWorkEntry(ctx, db, e); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	for _, tag := range []string{"perf", "caching"} {
		if err := AddTagToWorkEntry(ctx, db, e.ID, tag); err != nil {
			t.Fatalf("AddTagToWorkEntry %q: %v", tag, err)
		}
	}

	got, err := GetWorkEntry(ctx, db, e.ID)
	if err != nil {
		t.Fatalf("GetWorkEntry: %v", err)
	}
	want := []string{"caching", "perf"}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Errorf("got.Tags = %v, want %v", got.Tags, want)
	}
}
