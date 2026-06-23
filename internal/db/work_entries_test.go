package db

import (
	"context"
	"database/sql"
	"errors"
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
		Title:        "Investigate p99 spike",
		Description:  "look at metrics in the last 24h",
		Status:       core.StatusInProgress,
		StatusReason: "started today",
		PadPath:      "C:/scratch/p99",
	})

	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	var (
		gotTitle, gotStatus              string
		gotDesc, gotReason, gotPad       sql.NullString
		gotCreatedAtStr, gotUpdatedAtStr string
	)
	row := db.QueryRow(
		`SELECT title, description, status, status_reason, pad_path, created_at, updated_at
		 FROM work_entries WHERE id = ?`, entry.ID,
	)
	if err := row.Scan(&gotTitle, &gotDesc, &gotStatus, &gotReason, &gotPad, &gotCreatedAtStr, &gotUpdatedAtStr); err != nil {
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
	if !gotPad.Valid || gotPad.String != entry.PadPath {
		t.Errorf("pad_path = %+v, want %q", gotPad, entry.PadPath)
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
		   AND pad_path        IS NULL`,
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

// TestInsertWorkEntry_RejectsDuplicateTitle locks in the contract for
// the title UNIQUE constraint: a second insert with the same title
// returns ErrWorkEntryTitleTaken (errors.Is-friendly) so callers can
// surface a user-friendly message without parsing driver error text.
func TestInsertWorkEntry_RejectsDuplicateTitle(t *testing.T) {
	db := newTestDB(t)
	first := makeValidEntry(t, core.NewWorkEntryParams{Title: "Refactor auth flow"})
	if err := InsertWorkEntry(context.Background(), db, first); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	dup := makeValidEntry(t, core.NewWorkEntryParams{Title: "Refactor auth flow"})
	err := InsertWorkEntry(context.Background(), db, dup)
	if err == nil {
		t.Fatal("expected duplicate-title insert to fail, got nil")
	}
	if !errors.Is(err, ErrWorkEntryTitleTaken) {
		t.Errorf("err = %v, want ErrWorkEntryTitleTaken", err)
	}
}

// TestInsertWorkEntry_RejectsDuplicateTitleCaseInsensitive verifies
// the COLLATE NOCASE part of the constraint — "Foo" and "foo" must
// collide. Otherwise users could accidentally create near-duplicates.
func TestInsertWorkEntry_RejectsDuplicateTitleCaseInsensitive(t *testing.T) {
	db := newTestDB(t)
	first := makeValidEntry(t, core.NewWorkEntryParams{Title: "Refactor Auth Flow"})
	if err := InsertWorkEntry(context.Background(), db, first); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	dup := makeValidEntry(t, core.NewWorkEntryParams{Title: "refactor auth flow"})
	err := InsertWorkEntry(context.Background(), db, dup)
	if err == nil {
		t.Fatal("expected case-insensitive duplicate to fail, got nil")
	}
	if !errors.Is(err, ErrWorkEntryTitleTaken) {
		t.Errorf("err = %v, want ErrWorkEntryTitleTaken", err)
	}
}

// TestGetWorkEntry_Found round-trips a full entry through the
// database, exercising scanWorkEntry's NULL handling and timestamp
// parsing in one go.
func TestGetWorkEntry_Found(t *testing.T) {
	db := newTestDB(t)
	want := makeValidEntry(t, core.NewWorkEntryParams{
		Title:        "Investigate p99 spike",
		Description:  "look at metrics in the last 24h",
		Status:       core.StatusInProgress,
		StatusReason: "started today",
		PadPath:      "C:/scratch/p99",
	})
	if err := InsertWorkEntry(context.Background(), db, want); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	got, err := GetWorkEntry(context.Background(), db, want.ID)
	if err != nil {
		t.Fatalf("GetWorkEntry: %v", err)
	}

	if got.ID != want.ID || got.Title != want.Title ||
		got.Description != want.Description || got.Status != want.Status ||
		got.StatusReason != want.StatusReason || got.PadPath != want.PadPath {
		t.Errorf("entry fields mismatch:\n got=%+v\nwant=%+v", got, want)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt round-trip: got=%v want=%v", got.CreatedAt, want.CreatedAt)
	}
	if !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("UpdatedAt round-trip: got=%v want=%v", got.UpdatedAt, want.UpdatedAt)
	}
}

// TestGetWorkEntry_NotFound asserts the sentinel-error contract.
// Callers rely on errors.Is(err, ErrWorkEntryNotFound) to distinguish
// "no such row" from a real failure.
func TestGetWorkEntry_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := GetWorkEntry(context.Background(), db, "nope0")
	if err == nil {
		t.Fatal("expected error for missing id, got nil")
	}
	if !errors.Is(err, ErrWorkEntryNotFound) {
		t.Errorf("err = %v, want ErrWorkEntryNotFound", err)
	}
}

// TestListWorkEntries_Empty: an empty table returns an empty (or nil)
// slice with no error.
func TestListWorkEntries_Empty(t *testing.T) {
	db := newTestDB(t)

	got, err := ListWorkEntries(context.Background(), db)
	if err != nil {
		t.Fatalf("ListWorkEntries: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

// TestListWorkEntries_OrdersByCreatedAtDesc verifies the listing
// contract: newest first. Timestamps are set explicitly so the test
// doesn't depend on time.Now() ordering across rapid inserts.
func TestListWorkEntries_OrdersByCreatedAtDesc(t *testing.T) {
	db := newTestDB(t)
	base := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)

	// Insert in shuffled timestamp order to make sure DB-level ORDER BY
	// is what's doing the work (not insertion order).
	entries := []core.WorkEntry{
		{ID: "aaa01", Title: "middle", Status: core.StatusNew, CreatedAt: base.Add(1 * time.Hour), UpdatedAt: base.Add(1 * time.Hour)},
		{ID: "aaa02", Title: "oldest", Status: core.StatusNew, CreatedAt: base, UpdatedAt: base},
		{ID: "aaa03", Title: "newest", Status: core.StatusNew, CreatedAt: base.Add(2 * time.Hour), UpdatedAt: base.Add(2 * time.Hour)},
	}
	for _, e := range entries {
		if err := InsertWorkEntry(context.Background(), db, e); err != nil {
			t.Fatalf("InsertWorkEntry %s: %v", e.ID, err)
		}
	}

	got, err := ListWorkEntries(context.Background(), db)
	if err != nil {
		t.Fatalf("ListWorkEntries: %v", err)
	}

	wantTitles := []string{"newest", "middle", "oldest"}
	if len(got) != len(wantTitles) {
		t.Fatalf("len = %d, want %d", len(got), len(wantTitles))
	}
	for i, w := range wantTitles {
		if got[i].Title != w {
			t.Errorf("entry[%d].Title = %q, want %q", i, got[i].Title, w)
		}
	}
}

// TestDeleteWorkEntry_RemovesRow is the smoke test: a successful
// delete makes the row vanish so a follow-up GET returns
// ErrWorkEntryNotFound.
func TestDeleteWorkEntry_RemovesRow(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "doomed"})
	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	if err := DeleteWorkEntry(context.Background(), db, entry.ID); err != nil {
		t.Fatalf("DeleteWorkEntry: %v", err)
	}

	if _, err := GetWorkEntry(context.Background(), db, entry.ID); !errors.Is(err, ErrWorkEntryNotFound) {
		t.Errorf("post-delete GetWorkEntry err = %v, want ErrWorkEntryNotFound", err)
	}
}

// TestDeleteWorkEntry_NotFound: deleting a non-existent id must
// return a wrapped ErrWorkEntryNotFound so callers can treat
// "already gone" distinctly from a driver failure.
func TestDeleteWorkEntry_NotFound(t *testing.T) {
	db := newTestDB(t)

	err := DeleteWorkEntry(context.Background(), db, "nope0")
	if !errors.Is(err, ErrWorkEntryNotFound) {
		t.Errorf("err = %v, want ErrWorkEntryNotFound", err)
	}
}

// TestDeleteWorkEntry_CascadesToTagJoinRows verifies the
// work_entry_tags ON DELETE CASCADE: removing an entry must also
// remove its join rows, but must NOT remove the tag rows
// themselves — they are a shared vocabulary.
func TestDeleteWorkEntry_CascadesToTagJoinRows(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "tagged"})
	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	for _, name := range []string{"alpha", "beta"} {
		if err := AddTagToWorkEntry(context.Background(), db, entry.ID, name); err != nil {
			t.Fatalf("AddTagToWorkEntry(%s): %v", name, err)
		}
	}

	if err := DeleteWorkEntry(context.Background(), db, entry.ID); err != nil {
		t.Fatalf("DeleteWorkEntry: %v", err)
	}

	var joinCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM work_entry_tags WHERE work_entry_id = ?`, entry.ID,
	).Scan(&joinCount); err != nil {
		t.Fatalf("count join rows: %v", err)
	}
	if joinCount != 0 {
		t.Errorf("join rows after delete = %d, want 0 (cascade should have cleared them)", joinCount)
	}

	var tagCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM tags WHERE name IN ('alpha', 'beta')`,
	).Scan(&tagCount); err != nil {
		t.Fatalf("count tag rows: %v", err)
	}
	if tagCount != 2 {
		t.Errorf("tag vocabulary rows after delete = %d, want 2 (tags must not cascade)", tagCount)
	}
}

// TestDeleteWorkEntry_ClearsSelectedPointer verifies that the
// state.selected_work_entry_id FK with ON DELETE SET NULL fires
// when we delete the currently selected entry. Otherwise the
// selection would silently dangle.
func TestDeleteWorkEntry_ClearsSelectedPointer(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "selected-then-deleted"})
	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}
	if err := SelectWorkEntry(context.Background(), db, entry.ID); err != nil {
		t.Fatalf("SelectWorkEntry: %v", err)
	}

	if err := DeleteWorkEntry(context.Background(), db, entry.ID); err != nil {
		t.Fatalf("DeleteWorkEntry: %v", err)
	}

	var sel sql.NullString
	if err := db.QueryRow(`SELECT selected_work_entry_id FROM state WHERE id = 1`).Scan(&sel); err != nil {
		t.Fatalf("read selected: %v", err)
	}
	if sel.Valid {
		t.Errorf("selected_work_entry_id = %q after delete, want NULL", sel.String)
	}
}

func TestUpdateWorkEntryPad_SetsPathAndBumpsUpdatedAt(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "pad target"})
	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	newPad := "/tmp/some/pad"
	// Bump updated_at by a clear interval so the round-trip check
	// can verify the column actually changed.
	newUpdatedAt := entry.UpdatedAt.Add(time.Hour)
	if err := UpdateWorkEntryPad(context.Background(), db, entry.ID, newPad, newUpdatedAt); err != nil {
		t.Fatalf("UpdateWorkEntryPad: %v", err)
	}

	got, err := GetWorkEntry(context.Background(), db, entry.ID)
	if err != nil {
		t.Fatalf("GetWorkEntry: %v", err)
	}
	if got.PadPath != newPad {
		t.Errorf("PadPath = %q, want %q", got.PadPath, newPad)
	}
	if !got.UpdatedAt.Equal(newUpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, newUpdatedAt)
	}
}

func TestUpdateWorkEntryPad_EmptyStringClearsColumn(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "pad clear target"})
	entry.PadPath = "/tmp/keep-this-until-cleared"
	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	if err := UpdateWorkEntryPad(context.Background(), db, entry.ID, "", time.Now().UTC()); err != nil {
		t.Fatalf("UpdateWorkEntryPad clear: %v", err)
	}

	// Verify the column went to NULL (not empty string) — that's
	// the contract of nullableText, and other code paths rely on
	// the distinction.
	var pad sql.NullString
	if err := db.QueryRow(`SELECT pad_path FROM work_entries WHERE id = ?`, entry.ID).Scan(&pad); err != nil {
		t.Fatalf("read pad_path: %v", err)
	}
	if pad.Valid {
		t.Errorf("pad_path = %q after clear, want NULL", pad.String)
	}
}

func TestUpdateWorkEntryPad_NotFoundErr(t *testing.T) {
	db := newTestDB(t)
	err := UpdateWorkEntryPad(context.Background(), db, "ghost", "/anywhere", time.Now().UTC())
	if !errors.Is(err, ErrWorkEntryNotFound) {
		t.Errorf("err = %v, want ErrWorkEntryNotFound", err)
	}
}

func TestUpdateWorkEntryStatus_SetsStatusReasonAndBumpsUpdatedAt(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{Title: "status target"})
	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	newUpdatedAt := entry.UpdatedAt.Add(time.Hour)
	if err := UpdateWorkEntryStatus(context.Background(), db, entry.ID,
		core.StatusAbandoned, "descoped", newUpdatedAt); err != nil {
		t.Fatalf("UpdateWorkEntryStatus: %v", err)
	}

	got, err := GetWorkEntry(context.Background(), db, entry.ID)
	if err != nil {
		t.Fatalf("GetWorkEntry: %v", err)
	}
	if got.Status != core.StatusAbandoned {
		t.Errorf("Status = %q, want %q", got.Status, core.StatusAbandoned)
	}
	if got.StatusReason != "descoped" {
		t.Errorf("StatusReason = %q, want %q", got.StatusReason, "descoped")
	}
	if !got.UpdatedAt.Equal(newUpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, newUpdatedAt)
	}
}

func TestUpdateWorkEntryStatus_EmptyReasonClearsColumn(t *testing.T) {
	db := newTestDB(t)
	entry := makeValidEntry(t, core.NewWorkEntryParams{
		Title:        "reason clear target",
		Status:       core.StatusAbandoned,
		StatusReason: "to be cleared",
	})
	if err := InsertWorkEntry(context.Background(), db, entry); err != nil {
		t.Fatalf("InsertWorkEntry: %v", err)
	}

	if err := UpdateWorkEntryStatus(context.Background(), db, entry.ID,
		core.StatusCompleted, "", time.Now().UTC()); err != nil {
		t.Fatalf("UpdateWorkEntryStatus: %v", err)
	}

	// The reason column should go to NULL (not empty string), matching
	// the nullableText contract relied on by scanWorkEntry.
	var reason sql.NullString
	if err := db.QueryRow(`SELECT status_reason FROM work_entries WHERE id = ?`, entry.ID).Scan(&reason); err != nil {
		t.Fatalf("read status_reason: %v", err)
	}
	if reason.Valid {
		t.Errorf("status_reason = %q after clear, want NULL", reason.String)
	}
}

func TestUpdateWorkEntryStatus_NotFoundErr(t *testing.T) {
	db := newTestDB(t)
	err := UpdateWorkEntryStatus(context.Background(), db, "ghost",
		core.StatusCompleted, "", time.Now().UTC())
	if !errors.Is(err, ErrWorkEntryNotFound) {
		t.Errorf("err = %v, want ErrWorkEntryNotFound", err)
	}
}
