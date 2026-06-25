package db

// Package-level migration machinery for orbit's SQLite database.
//
// The public surface is [DumpSchema], which genschema and tests use to produce
// a normalised, comparable representation of whatever is in a DB.

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration holds the parsed content of a single numbered SQL migration file.
type migration struct {
	version  int
	filename string
	sql      string
}

// loadMigrations reads every *.sql file from the embedded migrations directory,
// parses each filename as N+_*.sql, and returns the migrations sorted by
// version. It rejects malformed filenames and duplicate version numbers.
func loadMigrations() ([]migration, error) {
	return loadMigrationsFrom(migrationsFS, "migrations")
}

// loadMigrationsFrom is the injectable core of loadMigrations. fsys must
// contain SQL files directly under dir/ named N+_*.sql.
func loadMigrationsFrom(fsys fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}

	seen := make(map[int]string) // version → filename, for duplicate detection
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		v, err := parseMigrationVersion(e.Name())
		if err != nil {
			return nil, err
		}
		if prev, ok := seen[v]; ok {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", v, prev, e.Name())
		}
		seen[v] = e.Name()

		content, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		out = append(out, migration{
			version:  v,
			filename: e.Name(),
			sql:      string(content),
		})
	}
	// Sort by parsed version number
	slices.SortFunc(out, func(a, b migration) int {
		switch {
		case a.version < b.version:
			return -1
		case a.version > b.version:
			return 1
		default:
			return 0
		}
	})
	return out, nil
}

// parseMigrationVersion extracts the integer version from a migration filename.
// Expected format: NNNN_short_description.sql (e.g. "0001_init.sql").
func parseMigrationVersion(filename string) (int, error) {
	idx := strings.IndexByte(filename, '_')
	if idx < 0 {
		return 0, fmt.Errorf("migration filename %q: expected NNNN_*.sql (no underscore found)", filename)
	}
	v, err := strconv.Atoi(filename[:idx])
	if err != nil {
		return 0, fmt.Errorf("migration filename %q: version prefix %q is not a number", filename, filename[:idx])
	}
	return v, nil
}

// DumpSchema returns a normalised, sorted snapshot of every non-internal
// object in db's sqlite_schema. Each element has the form "type|name|ddl"
// with internal whitespace collapsed.
//
// It is used by the genschema tool and by tests that compare schema snapshots.
func DumpSchema(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT type, name, COALESCE(sql, '') FROM sqlite_schema
		WHERE name NOT LIKE 'sqlite_%'
		ORDER BY type, name`)
	if err != nil {
		return nil, fmt.Errorf("query sqlite_schema: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var typ, name, ddl string
		if err := rows.Scan(&typ, &name, &ddl); err != nil {
			return nil, fmt.Errorf("scan sqlite_schema: %w", err)
		}
		out = append(out, fmt.Sprintf("%s|%s|%s", typ, name, strings.Join(strings.Fields(ddl), " ")))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite_schema: %w", err)
	}
	return out, nil
}
