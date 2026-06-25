package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
//	go run -tags ci ./cmd/genschema/
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
