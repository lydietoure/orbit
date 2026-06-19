package core

import (
	"strings"
	"testing"
	"time"
)

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
