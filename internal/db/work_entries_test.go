package db

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/lydietoure/orbit/internal/core"
)

// TestCreateWorkEntry_Defaults covers the happy path with only a title:
// ID is generated, status defaults to "new", timestamps are populated
// and equal, and the row is actually persisted.
func TestCreateWorkEntry_Defaults(t *testing.T) {
	db := newTestDB(t)
	before := time.Now().UTC().Add(-time.Second)

	entry, err := CreateWorkEntry(context.Background(), db, CreateWorkEntryParams{
		Title: "Add caching to payment flow",
	})
	if err != nil {
		t.Fatalf("CreateWorkEntry: %v", err)
	}

	if len(entry.ID) != 5 {
		t.Errorf("ID = %q (len %d), want 5 chars", entry.ID, len(entry.ID))
	}
	if entry.Status != core.StatusNew {
		t.Errorf("Status = %q, want %q", entry.Status, core.StatusNew)
	}
	if entry.CreatedAt.IsZero() || entry.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set: created=%v updated=%v", entry.CreatedAt, entry.UpdatedAt)
	}
	if !entry.CreatedAt.Equal(entry.UpdatedAt) {
		t.Errorf("CreatedAt (%v) and UpdatedAt (%v) should be equal on insert", entry.CreatedAt, entry.UpdatedAt)
	}
	if entry.CreatedAt.Before(before) {
		t.Errorf("CreatedAt %v is before test start %v", entry.CreatedAt, before)
	}

	// Confirm the row actually landed.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM work_entries WHERE id = ?`, entry.ID).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}

// TestCreateWorkEntry_AllFieldsRoundTrip stores every field and reads
// the row back to confirm each one survived the trip.
func TestCreateWorkEntry_AllFieldsRoundTrip(t *testing.T) {
	db := newTestDB(t)

	params := CreateWorkEntryParams{
		Title:          "Investigate p99 spike",
		Description:    "look at metrics in the last 24h",
		Status:         core.StatusInProgress,
		StatusReason:   "started today",
		ScratchpadPath: "C:/scratch/p99",
	}
	entry, err := CreateWorkEntry(context.Background(), db, params)
	if err != nil {
		t.Fatalf("CreateWorkEntry: %v", err)
	}

	var (
		gotTitle, gotStatus              string
		gotDesc, gotReason, gotScratch   sql.NullString
		gotCreatedAtStr, gotUpdatedAtStr string
	)
	row := db.QueryRow(
		`SELECT title, description, status, status_reason, scratchpad_path, created_at, updated_at
		 FROM work_entries WHERE id = ?`, entry.ID,
	)
	if err := row.Scan(&gotTitle, &gotDesc, &gotStatus, &gotReason, &gotScratch, &gotCreatedAtStr, &gotUpdatedAtStr); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if gotTitle != params.Title {
		t.Errorf("title = %q, want %q", gotTitle, params.Title)
	}
	if !gotDesc.Valid || gotDesc.String != params.Description {
		t.Errorf("description = %+v, want %q", gotDesc, params.Description)
	}
	if gotStatus != string(params.Status) {
		t.Errorf("status = %q, want %q", gotStatus, params.Status)
	}
	if !gotReason.Valid || gotReason.String != params.StatusReason {
		t.Errorf("status_reason = %+v, want %q", gotReason, params.StatusReason)
	}
	if !gotScratch.Valid || gotScratch.String != params.ScratchpadPath {
		t.Errorf("scratchpad_path = %+v, want %q", gotScratch, params.ScratchpadPath)
	}

	// Timestamps round-trip via RFC3339Nano.
	parsedCreated, err := time.Parse(time.RFC3339Nano, gotCreatedAtStr)
	if err != nil {
		t.Errorf("parse created_at %q: %v", gotCreatedAtStr, err)
	}
	if !parsedCreated.Equal(entry.CreatedAt) {
		t.Errorf("created_at round-trip mismatch: db=%v entry=%v", parsedCreated, entry.CreatedAt)
	}
}

// TestCreateWorkEntry_EmptyOptionalsBecomeNull verifies that empty
// strings for optional fields are stored as SQL NULL, not as the
// empty string. This keeps "absent" cleanly distinguishable in the DB.
func TestCreateWorkEntry_EmptyOptionalsBecomeNull(t *testing.T) {
	db := newTestDB(t)

	entry, err := CreateWorkEntry(context.Background(), db, CreateWorkEntryParams{
		Title: "minimal",
	})
	if err != nil {
		t.Fatalf("CreateWorkEntry: %v", err)
	}

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM work_entries
		 WHERE id = ?
		   AND description     IS NULL
		   AND status_reason   IS NULL
		   AND scratchpad_path IS NULL`,
		entry.ID,
	).Scan(&count); err != nil {
		t.Fatalf("count nulls: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d rows with all optionals NULL, want 1", count)
	}
}

// TestCreateWorkEntry_RejectsEmptyTitle covers the validation that a
// title (including a whitespace-only one) is required.
func TestCreateWorkEntry_RejectsEmptyTitle(t *testing.T) {
	db := newTestDB(t)

	for _, title := range []string{"", "   ", "\t\n"} {
		_, err := CreateWorkEntry(context.Background(), db, CreateWorkEntryParams{Title: title})
		if err == nil {
			t.Errorf("title %q: expected error, got nil", title)
		}
	}
}

// TestCreateWorkEntry_RejectsInvalidStatus rejects a status that is
// neither empty (defaults to new) nor one of the known values.
func TestCreateWorkEntry_RejectsInvalidStatus(t *testing.T) {
	db := newTestDB(t)

	_, err := CreateWorkEntry(context.Background(), db, CreateWorkEntryParams{
		Title:  "x",
		Status: core.WorkEntryStatus("blocked"),
	})
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error %q should mention the offending status", err)
	}
}

// TestCreateWorkEntry_RequiresReasonWhenAbandoned enforces the
// data-model rule that an abandoned entry needs a reason.
func TestCreateWorkEntry_RequiresReasonWhenAbandoned(t *testing.T) {
	db := newTestDB(t)

	// Abandoned with no reason — should fail.
	_, err := CreateWorkEntry(context.Background(), db, CreateWorkEntryParams{
		Title:  "x",
		Status: core.StatusAbandoned,
	})
	if err == nil {
		t.Fatal("expected error for abandoned without reason, got nil")
	}

	// Whitespace-only reason — should also fail.
	_, err = CreateWorkEntry(context.Background(), db, CreateWorkEntryParams{
		Title:        "x",
		Status:       core.StatusAbandoned,
		StatusReason: "   ",
	})
	if err == nil {
		t.Fatal("expected error for abandoned with blank reason, got nil")
	}

	// Abandoned with a real reason — should succeed.
	if _, err := CreateWorkEntry(context.Background(), db, CreateWorkEntryParams{
		Title:        "x",
		Status:       core.StatusAbandoned,
		StatusReason: "descoped",
	}); err != nil {
		t.Errorf("abandoned with reason should succeed, got %v", err)
	}
}

// TestCreateWorkEntry_GeneratesUniqueIDs is a smoke check that the
// caller does not need to coordinate IDs across calls.
func TestCreateWorkEntry_GeneratesUniqueIDs(t *testing.T) {
	db := newTestDB(t)

	const n = 16
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		entry, err := CreateWorkEntry(context.Background(), db, CreateWorkEntryParams{Title: "x"})
		if err != nil {
			t.Fatalf("CreateWorkEntry #%d: %v", i, err)
		}
		if seen[entry.ID] {
			t.Errorf("duplicate ID %q at iteration %d", entry.ID, i)
		}
		seen[entry.ID] = true
	}
}
