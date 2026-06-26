package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// openMemDB returns an open in-memory SQLite connection.
func openMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// dumpSchema returns one normalised string per object in sqlite_schema,
// excluding sqlite-internal objects. Each entry is "type|name|sql".
// Whitespace inside SQL is collapsed so minor formatting differences
// (extra spaces, newlines) don't produce false failures.
func dumpSchema(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`
		SELECT type, name, COALESCE(sql, '') FROM sqlite_schema
		WHERE name NOT LIKE 'sqlite_%'
		ORDER BY type, name`)
	if err != nil {
		t.Fatalf("query sqlite_schema: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var typ, name, ddl string
		if err := rows.Scan(&typ, &name, &ddl); err != nil {
			t.Fatalf("scan sqlite_schema row: %v", err)
		}
		// Collapse whitespace so formatting differences don't matter.
		normalised := strings.Join(strings.Fields(ddl), " ")
		out = append(out, fmt.Sprintf("%s|%s|%s", typ, name, normalised))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_schema: %v", err)
	}
	return out
}

// validFS returns a minimal fake FS with two well-formed migration files.
func validFS() fstest.MapFS {
	return fstest.MapFS{
		"migrations/0000_init.sql":    {Data: []byte("CREATE TABLE a (id INTEGER PRIMARY KEY);")},
		"migrations/0001_add_col.sql": {Data: []byte("ALTER TABLE a ADD COLUMN name TEXT;")},
	}
}

func TestLoadMigrationsFrom_HappyPath(t *testing.T) {
	ms, err := loadMigrationsFrom(validFS(), "migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(ms))
	}
	if ms[0].version != 0 || ms[0].filename != "0000_init.sql" {
		t.Errorf("ms[0]: got version=%d filename=%q", ms[0].version, ms[0].filename)
	}
	if ms[1].version != 1 || ms[1].filename != "0001_add_col.sql" {
		t.Errorf("ms[1]: got version=%d filename=%q", ms[1].version, ms[1].filename)
	}
}

func TestLoadMigrationsFrom_NonSQLFilesIgnored(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/0000_init.sql": {Data: []byte("CREATE TABLE a (id INTEGER PRIMARY KEY);")},
		"migrations/README.md":     {Data: []byte("not a migration")},
		"migrations/.gitkeep":      {Data: []byte{}},
	}
	ms, err := loadMigrationsFrom(fsys, "migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ms) != 1 {
		t.Errorf("expected 1 migration, got %d", len(ms))
	}
}

func TestLoadMigrationsFrom_MalformedFilename_NoUnderscore(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/init.sql": {Data: []byte("SELECT 1;")},
	}
	_, err := loadMigrationsFrom(fsys, "migrations")
	if err == nil {
		t.Fatal("expected error for filename without underscore, got nil")
	}
}

func TestLoadMigrationsFrom_MalformedFilename_NonNumericPrefix(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/abc_init.sql": {Data: []byte("SELECT 1;")},
	}
	_, err := loadMigrationsFrom(fsys, "migrations")
	if err == nil {
		t.Fatal("expected error for non-numeric version prefix, got nil")
	}
}

func TestLoadMigrationsFrom_DuplicateVersion(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/0001_first.sql":  {Data: []byte("SELECT 1;")},
		"migrations/0001_second.sql": {Data: []byte("SELECT 2;")},
	}
	_, err := loadMigrationsFrom(fsys, "migrations")
	if err == nil {
		t.Fatal("expected error for duplicate version, got nil")
	}
}

func TestLoadMigrationsFrom_SortedByVersion(t *testing.T) {
	// "2_foo.sql" sorts before "10_bar.sql" lexically but after numerically.
	// Also mixes zero-padded and non-padded prefixes.
	fsys := fstest.MapFS{
		"migrations/10_ten.sql": {Data: []byte("SELECT 10;")},
		"migrations/2_two.sql":  {Data: []byte("SELECT 2;")},
		"migrations/1_one.sql":  {Data: []byte("SELECT 1;")},
	}
	ms, err := loadMigrationsFrom(fsys, "migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int{1, 2, 10}
	for i, m := range ms {
		if m.version != want[i] {
			t.Errorf("ms[%d].version = %d, want %d", i, m.version, want[i])
		}
	}
}

func TestLoadMigrationsFrom_EmbeddedFS(t *testing.T) {
	// Smoke-test that loadMigrations() (the real embedded FS path) works
	// and returns at least one migration.
	ms, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(ms) == 0 {
		t.Fatal("expected at least one embedded migration, got none")
	}
	if ms[0].sql == "" {
		t.Error("first migration has empty SQL")
	}
}

// --- Migrate tests ---

func TestMigrate_FreshDB(t *testing.T) {
	db := openMemDB(t)
	if err := migrateFrom(db, validFS(), "migrations"); err != nil {
		t.Fatalf("migrateFrom: %v", err)
	}

	// Both migrations must be recorded.
	applied, err := appliedVersions(db)
	if err != nil {
		t.Fatalf("appliedVersions: %v", err)
	}
	if !applied[0] || !applied[1] {
		t.Errorf("expected versions 0 and 1 applied, got %v", applied)
	}

	// The schema produced by the fake FS should have the table and column.
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('a')`).Scan(&count); err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	if count != 2 { // id + name
		t.Errorf("expected 2 columns on table a, got %d", count)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	db := openMemDB(t)
	if err := migrateFrom(db, validFS(), "migrations"); err != nil {
		t.Fatalf("first migrateFrom: %v", err)
	}
	if err := migrateFrom(db, validFS(), "migrations"); err != nil {
		t.Fatalf("second migrateFrom: %v", err)
	}
	// Exactly 2 rows — no duplicates.
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 rows in schema_migrations after two runs, got %d", n)
	}
}

func TestMigrate_DBNewerThanBinary(t *testing.T) {
	db := openMemDB(t)
	// Bootstrap the table and stamp a version the fake FS doesn't know about.
	if err := ensureSchemaMigrationsTable(db); err != nil {
		t.Fatalf("ensureSchemaMigrationsTable: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (9999, '2099-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert sentinel: %v", err)
	}

	err := migrateFrom(db, validFS(), "migrations")
	if err == nil {
		t.Fatal("expected ErrSchemaDrift, got nil")
	}
	if !errors.Is(err, ErrSchemaDrift) {
		t.Errorf("expected ErrSchemaDrift, got %v", err)
	}
}

func TestMigrate_BadMigrationRollsBack(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/0000_init.sql": {Data: []byte("CREATE TABLE a (id INTEGER PRIMARY KEY);")},
		"migrations/0001_bad.sql":  {Data: []byte("THIS IS NOT VALID SQL !!!;")},
	}
	db := openMemDB(t)
	err := migrateFrom(db, fsys, "migrations")
	if err == nil {
		t.Fatal("expected error from bad SQL, got nil")
	}

	// Version 0 was committed before the bad migration ran; version 1 must not be recorded.
	applied, _ := appliedVersions(db)
	if applied[1] {
		t.Error("version 1 should not be recorded after a failed migration")
	}
	// Table `a` must still exist (migration 0 committed successfully).
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name = 'a'`).Scan(&n); err != nil {
		t.Fatalf("query sqlite_schema: %v", err)
	}
	if n != 1 {
		t.Error("table 'a' from migration 0 should still exist after migration 1 failed")
	}
}

// TestMigrate_AdoptLegacyV010DB verifies that Migrate can adopt a database
// created by orbit v0.1.0 (which used Initialize / user_version, with no
// schema_migrations table) without losing any existing data.
//
// The fixture at testdata/v0_1_0_app/home/orbit.db was generated by running
// the v0.1.0 binary against a fresh ORBIT_HOME and seeding some data.
func TestMigrate_AdoptLegacyV010DB(t *testing.T) {
	// Copy the fixture so we never modify the committed file.
	src := filepath.Join("testdata", "v0_1_0_app", "home", "orbit.db")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read legacy fixture: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "orbit.db")
	if err := os.WriteFile(dst, data, 0600); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}

	db, err := Open(dst)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Both migrations must be recorded in schema_migrations.
	applied, err := appliedVersions(db)
	if err != nil {
		t.Fatalf("appliedVersions: %v", err)
	}
	if !applied[0] || !applied[1] {
		t.Errorf("expected versions 0 and 1 applied, got %v", applied)
	}

	// Pre-existing data must survive.
	var nWork int
	if err := db.QueryRow(`SELECT count(*) FROM work_entries`).Scan(&nWork); err != nil {
		t.Fatalf("count work_entries: %v", err)
	}
	if nWork != 2 {
		t.Errorf("expected 2 work entries, got %d", nWork)
	}

	// Schema must match what applying all migrations to a fresh DB produces.
	freshDB := openMemDB(t)
	if err := Migrate(freshDB); err != nil {
		t.Fatalf("Migrate fresh db: %v", err)
	}
	wantSchema, err := DumpSchema(freshDB)
	if err != nil {
		t.Fatalf("DumpSchema fresh: %v", err)
	}
	
	gotSchema, err := DumpSchema(db)
	if err != nil {
		t.Fatalf("DumpSchema legacy: %v", err)
	}
	if len(wantSchema) != len(gotSchema) {
		t.Fatalf("schema length mismatch: want %d objects, got %d\nwant: %v\ngot:  %v",
			len(wantSchema), len(gotSchema), wantSchema, gotSchema)
	}
	for i := range wantSchema {
		if wantSchema[i] != gotSchema[i] {
			t.Errorf("schema[%d] mismatch:\nwant: %s\ngot:  %s", i, wantSchema[i], gotSchema[i])
		}
	}
}
