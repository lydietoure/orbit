package app

import (
	"context"
	"reflect"
	"testing"

	"github.com/lydietoure/orbit/internal/core"
)

// TestListTags_ExcludesReserved verifies `work tag list` shows the
// plain, free-form tags on an entry while leaving the reserved
// project:*/owner:* tags to their dedicated views.
func TestListTags_ExcludesReserved(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := newSelectedEntry(t, "tagged work")

	for _, raw := range []string{"perf", "caching"} {
		if _, _, err := TagWork(ctx, id, raw); err != nil {
			t.Fatalf("TagWork(%q): %v", raw, err)
		}
	}
	if _, _, err := AddProject(ctx, id, "payments"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, _, err := SetOwner(ctx, id, "work"); err != nil {
		t.Fatalf("SetOwner: %v", err)
	}

	resolved, tags, err := ListTags(ctx, "") // selected-entry fallback
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if resolved != id {
		t.Errorf("resolved id = %q, want %q", resolved, id)
	}
	want := []string{"caching", "perf"} // alphabetical, no reserved
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %v, want %v", tags, want)
	}
}

// TestListTags_EmptyState confirms an entry with no plain tags yields
// an empty result (not an error) so the CLI can print a clean message.
func TestListTags_EmptyState(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := newSelectedEntry(t, "no plain tags")
	// Only a reserved tag — `work tag list` should still come back empty.
	if _, _, err := SetOwner(ctx, id, "work"); err != nil {
		t.Fatalf("SetOwner: %v", err)
	}

	_, tags, err := ListTags(ctx, id)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("tags = %v, want empty", tags)
	}
}

// TestListAllTags_CountsAndExcludesReserved verifies `orbit tags`
// returns the plain tag vocabulary with per-tag counts, alphabetical,
// and hides reserved project:*/owner:* tags.
func TestListAllTags_CountsAndExcludesReserved(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()

	a := mustCreate(t, "alpha")
	b := mustCreate(t, "beta")
	for _, id := range []string{a, b} {
		if _, _, err := TagWork(ctx, id, "caching"); err != nil {
			t.Fatalf("TagWork: %v", err)
		}
	}
	if _, _, err := TagWork(ctx, a, "perf"); err != nil {
		t.Fatalf("TagWork: %v", err)
	}
	if _, _, err := AddProject(ctx, a, "payments"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, _, err := SetOwner(ctx, b, "work"); err != nil {
		t.Fatalf("SetOwner: %v", err)
	}

	got, err := ListAllTags(ctx)
	if err != nil {
		t.Fatalf("ListAllTags: %v", err)
	}
	want := []core.TagCount{
		{Name: "caching", Count: 2},
		{Name: "perf", Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListAllTags = %v, want %v", got, want)
	}
}

func TestListAllTags_EmptyIsNil(t *testing.T) {
	setupInitializedHome(t)
	got, err := ListAllTags(context.Background())
	if err != nil {
		t.Fatalf("ListAllTags: %v", err)
	}
	if got != nil {
		t.Errorf("ListAllTags = %v, want nil", got)
	}
}

// TestListWork_TagFilterAndSemantics verifies the repeatable --tag
// filter narrows results to entries carrying every requested tag, with
// case-insensitive normalization.
func TestListWork_TagFilterAndSemantics(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()

	a := mustCreate(t, "has both")
	b := mustCreate(t, "has caching only")
	_ = mustCreate(t, "has none")

	for _, raw := range []string{"caching", "perf"} {
		if _, _, err := TagWork(ctx, a, raw); err != nil {
			t.Fatalf("TagWork(%q): %v", raw, err)
		}
	}
	if _, _, err := TagWork(ctx, b, "caching"); err != nil {
		t.Fatalf("TagWork: %v", err)
	}

	// Single tag matches both a and b.
	got, err := ListWork(ctx, []string{"caching"})
	if err != nil {
		t.Fatalf("ListWork: %v", err)
	}
	if ids := idSet(got); !ids[a] || !ids[b] || len(ids) != 2 {
		t.Errorf("--tag caching ids = %v, want {%s, %s}", ids, a, b)
	}

	// AND semantics: both tags only match a. Mixed case must still match.
	got, err = ListWork(ctx, []string{"Caching", "perf"})
	if err != nil {
		t.Fatalf("ListWork: %v", err)
	}
	if ids := idSet(got); len(ids) != 1 || !ids[a] {
		t.Errorf("--tag Caching --tag perf ids = %v, want {%s}", ids, a)
	}

	// No filter returns everything.
	got, err = ListWork(ctx, nil)
	if err != nil {
		t.Fatalf("ListWork: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("unfiltered count = %d, want 3", len(got))
	}
}

// mustCreate makes a work entry without selecting it and returns its id.
func mustCreate(t *testing.T, title string) string {
	t.Helper()
	entry, err := CreateWork(context.Background(), CreateWorkParams{Title: title, NoSelect: true})
	if err != nil {
		t.Fatalf("CreateWork(%q): %v", title, err)
	}
	return entry.ID
}

func idSet(entries []core.WorkEntry) map[string]bool {
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.ID] = true
	}
	return out
}
