package cli

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
)

// newTestDB opens a fresh orbit DB in a per-test temp dir, applies the
// schema, and registers cleanup. Sibling helper to internal/db's
// version — kept package-local so cli tests don't need to reach into
// the db package's test helpers.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "orbit.db")

	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if err := db.Initialize(d); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return d
}

// TestRunWorkNew_CreatesEntryAndPrints confirms the happy path:
// a single-title call produces a persisted row with status "new" and
// a confirmation line on stdout.
func TestRunWorkNew_CreatesEntryAndPrints(t *testing.T) {
	d := newTestDB(t)
	var buf bytes.Buffer

	err := runWorkNew(context.Background(), d, &buf, workNewOpts{
		title: "Add caching to payment flow",
	})
	if err != nil {
		t.Fatalf("runWorkNew: %v", err)
	}

	// Output line: `Created <id>: "<title>"`
	out := buf.String()
	if !strings.HasPrefix(out, "Created ") {
		t.Errorf("output %q should start with `Created `", out)
	}
	if !strings.Contains(out, `"Add caching to payment flow"`) {
		t.Errorf("output %q should contain the quoted title", out)
	}

	// Confirm the row exists with status=new.
	var (
		gotTitle, gotStatus string
		gotCount            int
	)
	if err := d.QueryRow(`SELECT COUNT(*), MAX(title), MAX(status) FROM work_entries`).
		Scan(&gotCount, &gotTitle, &gotStatus); err != nil {
		t.Fatalf("query: %v", err)
	}
	if gotCount != 1 {
		t.Fatalf("row count = %d, want 1", gotCount)
	}
	if gotTitle != "Add caching to payment flow" {
		t.Errorf("title = %q, want full title", gotTitle)
	}
	if core.WorkEntryStatus(gotStatus) != core.StatusNew {
		t.Errorf("status = %q, want %q", gotStatus, core.StatusNew)
	}
}

// TestRunWorkNew_StoresOptionalFields exercises the --description and
// --scratchpad flags by checking that their values land in the DB.
func TestRunWorkNew_StoresOptionalFields(t *testing.T) {
	d := newTestDB(t)
	var buf bytes.Buffer

	err := runWorkNew(context.Background(), d, &buf, workNewOpts{
		title:       "Investigate p99 spike",
		description: "look at metrics in the last 24h",
		scratchpad:  "C:/scratch/p99",
	})
	if err != nil {
		t.Fatalf("runWorkNew: %v", err)
	}

	var (
		gotDesc, gotScratch sql.NullString
	)
	if err := d.QueryRow(
		`SELECT description, scratchpad_path FROM work_entries`,
	).Scan(&gotDesc, &gotScratch); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !gotDesc.Valid || gotDesc.String != "look at metrics in the last 24h" {
		t.Errorf("description = %+v, want the supplied text", gotDesc)
	}
	if !gotScratch.Valid || gotScratch.String != "C:/scratch/p99" {
		t.Errorf("scratchpad_path = %+v, want the supplied path", gotScratch)
	}
}

// TestRunWorkNew_PropagatesValidationErrors verifies that errors from
// db.CreateWorkEntry (e.g., empty title) reach the caller unchanged,
// no DB row is written, and nothing is printed.
func TestRunWorkNew_PropagatesValidationErrors(t *testing.T) {
	d := newTestDB(t)
	var buf bytes.Buffer

	err := runWorkNew(context.Background(), d, &buf, workNewOpts{title: "   "})
	if err == nil {
		t.Fatal("expected error for blank title, got nil")
	}

	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM work_entries`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Errorf("row count after failed create = %d, want 0", count)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output on error, got %q", buf.String())
	}
}
