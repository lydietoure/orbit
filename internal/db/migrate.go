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
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
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

// --- schema_migrations bookkeeping ---

func ensureSchemaMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at TEXT    NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	out := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		out[v] = true
	}
	return out, rows.Err()
}

func recordApplied(tx *sql.Tx, version int, at time.Time) error {
	_, err := tx.Exec(
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		version, at.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record migration %d: %w", version, err)
	}
	return nil
}

// --- Migrate ---

// Migrate applies every pending migration in internal/db/migrations/ to db
// in version order. It is idempotent: already-applied migrations are skipped.
//
// It returns [ErrSchemaDrift] when the database contains a migration version
// higher than the highest one embedded in this binary, meaning the DB was
// created by a newer orbit build. In that case the database is not touched.
//
func Migrate(db *sql.DB) error {
	return migrateFrom(db, migrationsFS, "migrations")
}

// migrateFrom is the injectable core of Migrate, used by tests.
func migrateFrom(db *sql.DB, fsys fs.FS, dir string) error {
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return err
	}

	all, err := loadMigrationsFrom(fsys, dir)
	if err != nil {
		return err
	}
	if len(all) == 0 {
		return nil
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	// Drift check: DB carries a version we don't know about.
	maxEmbedded := all[len(all)-1].version
	for v := range applied {
		if v > maxEmbedded {
			return fmt.Errorf(
				"%w: your data was created using a newer version of the app, and so it cannot read it — please update the app",
				ErrSchemaDrift)
		}
	}

	// Apply pending migrations.
	var nApplied int
	for _, m := range all {
		if applied[m.version] {
			continue
		}
		slog.Debug("applying migration", "version", m.version, "file", m.filename)
		if err := applyOneMigration(db, m); err != nil {
			return err
		}
		nApplied++
	}
	if nApplied > 0 {
		slog.Info("migrations applied", "count", nApplied)
	}
	return nil
}

func applyOneMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction for migration %s: %w", m.filename, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.Exec(m.sql); err != nil {
		return fmt.Errorf("exec migration %s: %w", m.filename, err)
	}
	if err := recordApplied(tx, m.version, time.Now()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", m.filename, err)
	}
	return nil
}

// IsLegacyDB reports whether db was created before migrations were released —
// i.e. it has a non-zero user_version but no schema_migrations table.
// Such databases are adoptable by Migrate without data loss.
func IsLegacyDB(db *sql.DB) (bool, error) {
	var userVersion int32
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		return false, err
	}
	if userVersion == 0 {
		return false, nil
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='schema_migrations'`).Scan(&n); err != nil {
		return false, err
	}
	return n == 0, nil
}