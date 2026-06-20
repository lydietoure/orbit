package core

import (
	"strings"
	"testing"
	"time"
)

// ---- NewWorkEntry ---------------------------------------------------------

// TestNewWorkEntry_Defaults covers the happy path with only a title:
// ID is generated, status defaults to "new", timestamps are populated
// and equal at construction.
func TestNewWorkEntry_Defaults(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)

	entry, err := NewWorkEntry(NewWorkEntryParams{Title: "  Add caching  "})
	if err != nil {
		t.Fatalf("NewWorkEntry: %v", err)
	}

	if entry.Title != "Add caching" {
		t.Errorf("title = %q, want trimmed %q", entry.Title, "Add caching")
	}
	if len(entry.ID) != 5 {
		t.Errorf("ID = %q (len %d), want 5 chars", entry.ID, len(entry.ID))
	}
	if entry.Status != StatusNew {
		t.Errorf("Status = %q, want %q", entry.Status, StatusNew)
	}
	if entry.CreatedAt.IsZero() || entry.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set: created=%v updated=%v", entry.CreatedAt, entry.UpdatedAt)
	}
	if !entry.CreatedAt.Equal(entry.UpdatedAt) {
		t.Errorf("CreatedAt (%v) and UpdatedAt (%v) should be equal at construction",
			entry.CreatedAt, entry.UpdatedAt)
	}
	if entry.CreatedAt.Before(before) {
		t.Errorf("CreatedAt %v is before test start %v", entry.CreatedAt, before)
	}
}

// TestNewWorkEntry_PreservesAllFields confirms every supplied field
// survives onto the resulting WorkEntry unchanged (except Title, which
// is trimmed).
func TestNewWorkEntry_PreservesAllFields(t *testing.T) {
	p := NewWorkEntryParams{
		Title:        "Investigate p99 spike",
		Description:  "look at metrics in the last 24h",
		Status:       StatusInProgress,
		StatusReason: "started today",
		PadPath:      "C:/scratch/p99",
	}
	entry, err := NewWorkEntry(p)
	if err != nil {
		t.Fatalf("NewWorkEntry: %v", err)
	}
	if entry.Title != p.Title {
		t.Errorf("title = %q, want %q", entry.Title, p.Title)
	}
	if entry.Description != p.Description {
		t.Errorf("description = %q, want %q", entry.Description, p.Description)
	}
	if entry.Status != p.Status {
		t.Errorf("status = %q, want %q", entry.Status, p.Status)
	}
	if entry.StatusReason != p.StatusReason {
		t.Errorf("status_reason = %q, want %q", entry.StatusReason, p.StatusReason)
	}
	if entry.PadPath != p.PadPath {
		t.Errorf("pad_path = %q, want %q", entry.PadPath, p.PadPath)
	}
}

// TestNewWorkEntry_RejectsEmptyTitle covers the validation that a
// title (including a whitespace-only one) is required.
func TestNewWorkEntry_RejectsEmptyTitle(t *testing.T) {
	for _, title := range []string{"", "   ", "\t\n"} {
		if _, err := NewWorkEntry(NewWorkEntryParams{Title: title}); err == nil {
			t.Errorf("title %q: expected error, got nil", title)
		}
	}
}

// TestNewWorkEntry_RejectsInvalidStatus rejects a status that is
// neither empty (defaults to new) nor one of the known values.
func TestNewWorkEntry_RejectsInvalidStatus(t *testing.T) {
	_, err := NewWorkEntry(NewWorkEntryParams{Title: "x", Status: WorkEntryStatus("blocked")})
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error %q should mention the offending status", err)
	}
}

// TestNewWorkEntry_RequiresReasonWhenAbandoned enforces the
// data-model rule that an abandoned entry needs a reason.
func TestNewWorkEntry_RequiresReasonWhenAbandoned(t *testing.T) {
	// No reason — fails.
	if _, err := NewWorkEntry(NewWorkEntryParams{Title: "x", Status: StatusAbandoned}); err == nil {
		t.Fatal("expected error for abandoned without reason, got nil")
	}
	// Whitespace-only reason — also fails.
	_, err := NewWorkEntry(NewWorkEntryParams{
		Title:        "x",
		Status:       StatusAbandoned,
		StatusReason: "   ",
	})
	if err == nil {
		t.Fatal("expected error for abandoned with blank reason, got nil")
	}
	// With a real reason — succeeds.
	if _, err := NewWorkEntry(NewWorkEntryParams{
		Title:        "x",
		Status:       StatusAbandoned,
		StatusReason: "descoped",
	}); err != nil {
		t.Errorf("abandoned with reason should succeed, got %v", err)
	}
}

// ---- NewID ----------------------------------------------------------------

func TestNewID_LengthAndAlphabet(t *testing.T) {
	for i := 0; i < 64; i++ {
		id := NewID()
		if len(id) != 5 {
			t.Fatalf("len(id) = %d, want 5 (id=%q)", len(id), id)
		}
		for _, ch := range id {
			if !strings.ContainsRune(idAlphabet, ch) {
				t.Fatalf("id %q contains char %q not in Crockford alphabet", id, ch)
			}
		}
	}
}

func TestNewIDAt_TimeSortable(t *testing.T) {
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// Force random bits to zero so ordering depends purely on the
	// time prefix.
	id1 := NewIDAt(base, 0)
	id2 := NewIDAt(base.Add(1*time.Hour), 0)
	id3 := NewIDAt(base.Add(24*time.Hour), 0)
	id4 := NewIDAt(base.Add(30*24*time.Hour), 0)

	if !(id1 < id2 && id2 < id3 && id3 < id4) {
		t.Errorf("expected chronological sort, got %q %q %q %q", id1, id2, id3, id4)
	}
}

func TestNewIDAt_DeterministicForSameInputs(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	a := NewIDAt(now, 42)
	b := NewIDAt(now, 42)
	if a != b {
		t.Errorf("same inputs gave different IDs: %q vs %q", a, b)
	}
}

func TestNewIDAt_DifferentRandBitsDifferIDs(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	a := NewIDAt(now, 0)
	b := NewIDAt(now, 1)
	if a == b {
		t.Errorf("different random bits should differ in suffix, both = %q", a)
	}
}

func TestNewIDAt_RandBitsAboveMaskAreIgnored(t *testing.T) {
	// Only the low idRandBits should influence output, so setting
	// any high bit alongside the same low bits should not change
	// the ID.
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	a := NewIDAt(now, 0x000003FF) // all 10 low bits set
	b := NewIDAt(now, 0xFFFFFFFF) // all 32 bits set
	if a != b {
		t.Errorf("expected high random bits to be ignored: %q vs %q", a, b)
	}
}

func TestNewID_CollisionsRareInTightLoop(t *testing.T) {
	// 64 IDs in one hour bucket → birthday paradox over 1024 slots
	// expects roughly 2 collisions; allow up to 8 before we suspect
	// the random source is broken.
	const n = 64
	seen := make(map[string]bool, n)
	collisions := 0
	for i := 0; i < n; i++ {
		id := NewID()
		if seen[id] {
			collisions++
		}
		seen[id] = true
	}
	if collisions > 8 {
		t.Errorf("got %d collisions in %d IDs; random source looks broken", collisions, n)
	}
}

func TestWorkEntryStatus_Valid(t *testing.T) {
	valid := []WorkEntryStatus{StatusNew, StatusInProgress, StatusCompleted, StatusAbandoned}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	invalid := []WorkEntryStatus{
		"",
		"active",
		"done",
		"dropped",
		"NEW",
		"In-Progress",
	}
	for _, s := range invalid {
		if s.Valid() {
			t.Errorf("%q should NOT be valid", s)
		}
	}
}
