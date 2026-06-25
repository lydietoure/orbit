package db

import (
	"database/sql"
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

// TestGoldenSchema applies every migration in order to an in-memory DB and
// compares the resulting schema against testdata/schema.golden.sql.
//
// Regenerate the golden file after changing migrations:
//
//	go run ./internal/db/genschema.go
func TestGoldenSchema(t *testing.T) {
	golden := filepath.Join("testdata", "schema.golden.sql")
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file %s: %v\n\tregenerate with: go run ./internal/db/genschema.go", golden, err)
	}

	db := openMemDB(t)

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		sql, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if _, err := db.Exec(string(sql)); err != nil {
			t.Fatalf("exec %s: %v", e.Name(), err)
		}
	}

	rows := dumpSchema(t, db)
	got := strings.Join(rows, "\n") + "\n"

	if got != string(want) {
		t.Errorf("schema differs from %s\n\tregenerate with: go run ./internal/db/genschema.go\n\ngot:\n%s\nwant:\n%s",
			golden, got, string(want))
	}
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
