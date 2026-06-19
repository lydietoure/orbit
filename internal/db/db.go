// Package db opens the orbit SQLite database and applies its embedded schema.
// It uses the pure-Go modernc.org/sqlite driver and sets per-connection
// pragmas via the DSN.
package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Open opens (or creates) the orbit database at the given path.
//
// It sets per-connection pragmas (foreign_keys, journal_mode=WAL, busy_timeout)
// via the DSN so they apply to every connection in the pool — post-open
// PRAGMA statements would only affect one connection.
//
// Open does NOT apply the schema (see Initialize separately) and does NOT create parent directories.
//
// The path must not contain a literal '?' character; it would be parsed
// as the DSN query separator.
func Open(path string) (*sql.DB, error) {
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

// Initialize applies the embedded schema to the database. Idempotent:
// every statement uses CREATE TABLE IF NOT EXISTS, so calling on an
// already-initialized DB is a no-op.
func Initialize(db *sql.DB) error {
	slog.Debug("applying schema", "bytes", len(schemaSQL))
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
