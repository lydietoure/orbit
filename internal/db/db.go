// Package db opens the orbit SQLite database and applies migrations.
// It uses the pure-Go modernc.org/sqlite driver and sets per-connection
// pragmas via the DSN.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	_ "modernc.org/sqlite"
)

// ErrSchemaDrift is returned by [Migrate] when the database contains a
// migration version higher than the highest one embedded in this binary,
// meaning the DB was created by a newer orbit build.
var ErrSchemaDrift = errors.New("database schema is out of date")

// Open opens (or creates) the orbit database at the given path.
//
// It sets per-connection pragmas (foreign_keys, journal_mode=WAL, busy_timeout)
// via the DSN so they apply to every connection in the pool — post-open
// PRAGMA statements would only affect one connection.
//
// Open does NOT apply migrations (see Migrate separately) and does NOT create parent directories.
//
// The path must not contain a literal '?' character; it would be parsed
// as the DSN query separator. Open rejects such paths with an error
// rather than letting them produce a malformed DSN and a confusing
// downstream failure.
func Open(path string) (*sql.DB, error) {
	if strings.Contains(path, "?") {
		return nil, fmt.Errorf("invalid database path %q: must not contain '?' (reserved as the DSN query separator)", path)
	}
	dsn := path + "?_pragma=foreign_keys(on)&_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)"
	slog.Debug("opening sqlite", "path", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite at %q: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite at %q: %w", path, err)
	}
	return db, nil
}
