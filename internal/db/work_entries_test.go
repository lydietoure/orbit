package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/lydietoure/orbit/internal/core"
)

// makeValidEntry builds a fully-populated WorkEntry via core.NewWorkEntry
// for tests that need a record to insert. Failing here keeps the
// test bodies focused on the DB-level assertions.
func makeValidEntry(t *testing.T, p core.NewWorkEntryParams) core.WorkEntry {
	t.Helper()
	entry, err := core.NewWorkEntry(p)
	if err != nil {
		t.Fatalf("core.NewWorkEntry: %v", err)
	}
	return entry
}

// TestInsertWorkEntry_Persists is the smoke test: inserting a valid
// entry results in exactly one row visible by ID.
func TestInsertWorkEntry_Persists(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "Add caching"})

	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM work_entries WHERE id = ?`, entry.ID).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}

// TestInsertWorkEntry_AllFieldsRoundTrip stores every field and reads
// the row back to confirm each one survived the trip.
func TestInsertWorkEntry_AllFieldsRoundTrip(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{
		Title:          "Investigate p99 spike",
		Description:    "look at metrics in the last 24h",
		Status:         core.StatusInProgress,
		StatusReason:   "started today",
		ScratchpadPath: "C:/scratch/p99",
	})

	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
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

	if gotTitle != entry.Title {
		t.Errorf("title = %q, want %q", gotTitle, entry.Title)
	}
	if !gotDesc.Valid || gotDesc.String != entry.Description {
		t.Errorf("description = %+v, want %q", gotDesc, entry.Description)
	}
	if gotStatus != string(entry.Status) {
		t.Errorf("status = %q, want %q", gotStatus, entry.Status)
	}
	if !gotReason.Valid || gotReason.String != entry.StatusReason {
		t.Errorf("status_reason = %+v, want %q", gotReason, entry.StatusReason)
	}
	if !gotScratch.Valid || gotScratch.String != entry.ScratchpadPath {
		t.Errorf("scratchpad_path = %+v, want %q", gotScratch, entry.ScratchpadPath)
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

// TestInsertWorkEntry_EmptyOptionalsBecomeNull verifies that empty
// strings for optional fields are stored as SQL NULL, not as the
// empty string. This keeps "absent" cleanly distinguishable in the DB.
func TestInsertWorkEntry_EmptyOptionalsBecomeNull(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "minimal"})

	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
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

// TestInsertWorkEntry_RejectsDuplicateID confirms the PRIMARY KEY
// constraint surfaces as an error from InsertWorkEntry — the second
// insert with the same ID must fail.
func TestInsertWorkEntry_RejectsDuplicateID(t *testing.T) {
	db := newTestDB(t)
	first := makeValidEntry(t, core.NewWorkEntryParams{Title: "first"})

	if err := InsertWorkEntry(context.Background(), db, first); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Build a second entry, then force its ID to collide.
	second := makeValidEntry(t, core.NewWorkEntryParams{Title: "second"})
	second.ID = first.ID
	if err := InsertWorkEntry(context.Background(), db, second); err == nil {
		t.Fatal("expected duplicate-ID insert to fail, got nil")
	}
}
