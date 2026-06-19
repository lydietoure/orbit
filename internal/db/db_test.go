package db

import (
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func init() {
	// Silence slog output during tests; warnings/errors from db.go would
	// otherwise clutter `go test` output. Discard via io.Discard works on
	// any Go version; once we standardize on Go 1.24+ for tooling we can
	// swap to slog.DiscardHandler if preferred.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// newTestDB opens a fresh orbit DB in a per-test temp dir, applies the
// schema, and registers cleanup. Returns a ready-to-use *sql.DB.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir() // auto-deleted at test end
	path := filepath.Join(dir, "orbit.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Initialize(db); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return db
}

// --- tests -----------------------------------------------------------------

// TestOpen_CreatesUsableDB confirms Open succeeds on a non-existent path
// (SQLite creates the file on first use) and the returned handle pings.
func TestOpen_CreatesUsableDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orbit.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

// TestInitialize_CreatesTables verifies that applying the schema produces
// exactly the M0 set of tables (ignoring SQLite's internal bookkeeping).
func TestInitialize_CreatesTables(t *testing.T) {
	db := newTestDB(t)

	rows, err := db.Query(
		`SELECT name FROM sqlite_master
         WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
         ORDER BY name`,
	)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	want := []string{"state", "tags", "work_entries", "work_entry_tags"}
	if !slices.Equal(got, want) {
		t.Errorf("tables = %v, want %v", got, want)
	}
}

// TestInitialize_IsIdempotent confirms running the schema twice does not
// error — required because Initialize runs on every DB open, not just init.
func TestInitialize_IsIdempotent(t *testing.T) {
	db := newTestDB(t) // first apply
	if err := Initialize(db); err != nil {
		t.Fatalf("second Initialize: %v", err)
	}
}

// TestStateSeeded verifies the singleton state row is present after init.
func TestStateSeeded(t *testing.T) {
	db := newTestDB(t)

	var id int
	err := db.QueryRow(`SELECT id FROM state`).Scan(&id)
	if err != nil {
		t.Fatalf("select state: %v", err)
	}
	if id != 1 {
		t.Errorf("state.id = %d, want 1", id)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM state`).Scan(&count); err != nil {
		t.Fatalf("count state: %v", err)
	}
	if count != 1 {
		t.Errorf("state row count = %d, want 1", count)
	}
}

// TestForeignKeysEnforced is the crucial test that the DSN pragma
// actually took effect: inserting a join row with a dangling FK must error.
// Without `_pragma=foreign_keys(on)` this insert would silently succeed.
func TestForeignKeysEnforced(t *testing.T) {
	db := newTestDB(t)

	_, err := db.Exec(
		`INSERT INTO work_entry_tags (work_entry_id, tag_id) VALUES (?, ?)`,
		"w-nope", 9999,
	)
	if err == nil {
		t.Fatal("expected FK violation, got nil error — pragma did not take effect")
	}
}

// TestCascadeDelete proves ON DELETE CASCADE actually fires (which only
// works because FKs are enforced — see TestForeignKeysEnforced).
func TestCascadeDelete(t *testing.T) {
	db := newTestDB(t)

	// Seed: one work entry, one tag, one join row.
	const id = "w-test"
	_, err := db.Exec(
		`INSERT INTO work_entries (id, title, created_at, updated_at)
         VALUES (?, ?, '2026-06-19T00:00:00Z', '2026-06-19T00:00:00Z')`,
		id, "test",
	)
	if err != nil {
		t.Fatalf("insert work_entry: %v", err)
	}

	res, err := db.Exec(`INSERT INTO tags (name) VALUES (?)`, "caching")
	if err != nil {
		t.Fatalf("insert tag: %v", err)
	}
	tagID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("lastInsertId: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO work_entry_tags (work_entry_id, tag_id) VALUES (?, ?)`,
		id, tagID,
	); err != nil {
		t.Fatalf("insert join: %v", err)
	}

	// Act: delete the work entry. The join row should cascade out.
	if _, err := db.Exec(`DELETE FROM work_entries WHERE id = ?`, id); err != nil {
		t.Fatalf("delete work_entry: %v", err)
	}

	// Assert.
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM work_entry_tags WHERE work_entry_id = ?`, id,
	).Scan(&count); err != nil {
		t.Fatalf("count join rows: %v", err)
	}
	if count != 0 {
		t.Errorf("join rows after delete = %d, want 0 — cascade did not fire", count)
	}
}

// TestInitialize_StampsVersionOnFreshDB confirms a brand-new database
// (user_version == 0) gets the current schema fingerprint stamped on
// it by Initialize.
func TestInitialize_StampsVersionOnFreshDB(t *testing.T) {
	db := newTestDB(t)

	var got int32
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&got); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if got != schemaVersion {
		t.Errorf("user_version = %d, want %d", got, schemaVersion)
	}
}

// TestInitialize_NoOpWhenVersionMatches verifies that re-running
// Initialize on a DB already stamped with the current version is a
// silent no-op (no error, no overwrite).
func TestInitialize_NoOpWhenVersionMatches(t *testing.T) {
	db := newTestDB(t) // already stamped by newTestDB

	if err := Initialize(db); err != nil {
		t.Fatalf("second Initialize: %v", err)
	}

	var got int32
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&got); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if got != schemaVersion {
		t.Errorf("user_version drifted to %d after no-op Initialize, want %d", got, schemaVersion)
	}
}

// TestInitialize_DetectsSchemaDrift is the regression guard for the
// bug that motivated this whole feature: an existing DB whose schema
// pre-dates the current binary must be refused with ErrSchemaDrift
// instead of silently running against the stale schema.
//
// We simulate drift by stamping a deliberately-wrong user_version on
// a freshly-initialized DB, then calling Initialize again.
func TestInitialize_DetectsSchemaDrift(t *testing.T) {
	db := newTestDB(t)

	// Stamp a different version to simulate a DB created from an
	// older (or just-different) schema source.
	if _, err := db.Exec(`PRAGMA user_version = 42`); err != nil {
		t.Fatalf("seed drifted version: %v", err)
	}

	err := Initialize(db)
	if err == nil {
		t.Fatal("expected ErrSchemaDrift, got nil")
	}
	if !errors.Is(err, ErrSchemaDrift) {
		t.Errorf("err = %v, want ErrSchemaDrift", err)
	}
	// User-facing message should tell them how to recover.
	msg := err.Error()
	if !strings.Contains(msg, "orbit destroy") || !strings.Contains(msg, "orbit init") {
		t.Errorf("error message %q should mention `orbit destroy` and `orbit init`", msg)
	}
}

// contains is a tiny strings.Contains shim so the test stays self-
// contained without adding an import just for one assertion.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
