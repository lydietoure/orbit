package app

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/lydietoure/orbit/internal/db"
)

// newSelectedEntry creates a work entry and leaves it selected, so the
// reserved-tag use cases under test can be exercised with an empty id
// (the selected-entry fallback). Returns the entry id.
func newSelectedEntry(t *testing.T, title string) string {
	t.Helper()
	entry, err := CreateWork(context.Background(), CreateWorkParams{Title: title})
	if err != nil {
		t.Fatalf("CreateWork: %v", err)
	}
	return entry.ID
}

func TestAddProject_MultipleAllowedAndIdempotent(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := newSelectedEntry(t, "projects work")

	for _, name := range []string{"payments", "orbit", "payments"} {
		if _, _, err := AddProject(ctx, "", name); err != nil {
			t.Fatalf("AddProject(%q): %v", name, err)
		}
	}

	resolved, projects, err := ListProjects(ctx, id)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if resolved != id {
		t.Errorf("resolved id = %q, want %q", resolved, id)
	}
	want := []string{"orbit", "payments"} // alphabetical, deduped
	if len(projects) != len(want) {
		t.Fatalf("projects = %v, want %v", projects, want)
	}
	for i := range want {
		if projects[i] != want[i] {
			t.Errorf("projects[%d] = %q, want %q", i, projects[i], want[i])
		}
	}
}

func TestRemoveProject_DropsOnlyThatProject(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := newSelectedEntry(t, "remove project")

	if _, _, err := AddProject(ctx, id, "payments"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, _, err := AddProject(ctx, id, "orbit"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, _, err := RemoveProject(ctx, id, "payments"); err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}

	_, projects, err := ListProjects(ctx, id)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 || projects[0] != "orbit" {
		t.Errorf("projects = %v, want [orbit]", projects)
	}
}

func TestRemoveProject_NotPresentReturnsErr(t *testing.T) {
	setupInitializedHome(t)
	id := newSelectedEntry(t, "no such project")
	if _, _, err := RemoveProject(context.Background(), id, "ghost"); !errors.Is(err, db.ErrTagNotOnEntry) {
		t.Fatalf("RemoveProject: got %v, want ErrTagNotOnEntry", err)
	}
}

func TestSetOwner_SingleValuedSwapsNeverAccumulates(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := newSelectedEntry(t, "owner swap")

	if _, _, err := SetOwner(ctx, "", "work"); err != nil {
		t.Fatalf("SetOwner(work): %v", err)
	}
	if _, owner, err := SetOwner(ctx, "", "personal"); err != nil {
		t.Fatalf("SetOwner(personal): %v", err)
	} else if owner != "personal" {
		t.Errorf("SetOwner returned owner %q, want %q", owner, "personal")
	}

	// Exactly one owner tag must remain, and it must be the new value.
	entry, err := db.GetWorkEntry(ctx, mustOpen(t), id)
	if err != nil {
		t.Fatalf("GetWorkEntry: %v", err)
	}
	owners := 0
	for _, tag := range entry.Tags {
		if tag == "owner:work" || tag == "owner:personal" {
			owners++
		}
	}
	if owners != 1 {
		t.Errorf("owner tags = %d (%v), want exactly 1", owners, entry.Tags)
	}

	_, owner, err := GetOwner(ctx, id)
	if err != nil {
		t.Fatalf("GetOwner: %v", err)
	}
	if owner != "personal" {
		t.Errorf("GetOwner = %q, want %q", owner, "personal")
	}
}

func TestSetOwner_ResetToSameValueIsIdempotent(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := newSelectedEntry(t, "owner idempotent")

	if _, _, err := SetOwner(ctx, id, "work"); err != nil {
		t.Fatalf("SetOwner: %v", err)
	}
	if _, _, err := SetOwner(ctx, id, "work"); err != nil {
		t.Fatalf("SetOwner (repeat): %v", err)
	}

	_, owner, err := GetOwner(ctx, id)
	if err != nil {
		t.Fatalf("GetOwner: %v", err)
	}
	if owner != "work" {
		t.Errorf("GetOwner = %q, want %q", owner, "work")
	}
}

func TestClearOwner_ReportsPreviousAndIsNoOpWhenAbsent(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	id := newSelectedEntry(t, "owner clear")

	// Clearing with no owner set returns an empty previous value.
	if _, prev, err := ClearOwner(ctx, id); err != nil {
		t.Fatalf("ClearOwner (empty): %v", err)
	} else if prev != "" {
		t.Errorf("ClearOwner previous = %q, want empty", prev)
	}

	if _, _, err := SetOwner(ctx, id, "work"); err != nil {
		t.Fatalf("SetOwner: %v", err)
	}
	if _, prev, err := ClearOwner(ctx, id); err != nil {
		t.Fatalf("ClearOwner: %v", err)
	} else if prev != "work" {
		t.Errorf("ClearOwner previous = %q, want %q", prev, "work")
	}

	if _, owner, err := GetOwner(ctx, id); err != nil {
		t.Fatalf("GetOwner: %v", err)
	} else if owner != "" {
		t.Errorf("GetOwner after clear = %q, want empty", owner)
	}
}

func TestReservedTags_NoSelectionReturnsHelpfulError(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	// Nothing created/selected.
	if _, _, err := AddProject(ctx, "", "payments"); !errors.Is(err, ErrNoTargetWorkEntry) {
		t.Errorf("AddProject with no selection: got %v, want ErrNoTargetWorkEntry", err)
	}
	if _, _, err := SetOwner(ctx, "", "work"); !errors.Is(err, ErrNoTargetWorkEntry) {
		t.Errorf("SetOwner with no selection: got %v, want ErrNoTargetWorkEntry", err)
	}
}

func TestReservedTags_UnknownIDReturnsNotFound(t *testing.T) {
	setupInitializedHome(t)
	ctx := context.Background()
	if _, _, err := AddProject(ctx, "zzzzz", "payments"); !errors.Is(err, db.ErrWorkEntryNotFound) {
		t.Errorf("AddProject unknown id: got %v, want ErrWorkEntryNotFound", err)
	}
	if _, _, err := SetOwner(ctx, "zzzzz", "work"); !errors.Is(err, db.ErrWorkEntryNotFound) {
		t.Errorf("SetOwner unknown id: got %v, want ErrWorkEntryNotFound", err)
	}
}

// mustOpen opens the configured DB for assertions that need to read raw
// rows. The caller is a test, so a failure to open is fatal.
func mustOpen(t *testing.T) *sql.DB {
	t.Helper()
	d, closer, err := open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(closer)
	return d
}
