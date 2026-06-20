// Package db opens the orbit SQLite database and applies its embedded schema.
// It uses the pure-Go modernc.org/sqlite driver and sets per-connection
// pragmas via the DSN.
package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"errors"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// schemaVersion is a fingerprint of the embedded schema source — the
// low 31 bits of SHA-256(schemaSQL), stuffed into SQLite's built-in
// PRAGMA user_version slot (a single signed-int32 in the DB header,
// no extra table needed). We mask the sign bit to stay positive.
//
// 31 bits gives ~1-in-2-billion accidental-collision odds across
// arbitrary schema edits, which is more than enough for "did this
// file change since the DB was created" — the only failure mode is
// "user edits schema in a way that happens to hash-collide", which
// is effectively a non-event for human-written SQL.
var schemaVersion = func() int32 {
	sum := sha256.Sum256([]byte(schemaSQL))
	v := binary.BigEndian.Uint32(sum[:4]) & 0x7FFF_FFFF
	return int32(v)
}()

// ErrSchemaDrift is returned by [Initialize] when the database was
// created from a different schema version than the binary embeds.
// It signals that the on-disk schema is out of sync with the
// expected one (constraints, columns, etc. may differ).
//
// M0 has no migrations, so the only resolution is to destroy and
// recreate the database. The error message tells the user as much.
var ErrSchemaDrift = errors.New("database schema is out of date")

// Open opens (or creates) the orbit database at the given path.
//
// It sets per-connection pragmas (foreign_keys, journal_mode=WAL, busy_timeout)
// via the DSN so they apply to every connection in the pool — post-open
// PRAGMA statements would only affect one connection.
//
// Open does NOT apply the schema (see Initialize separately) and does NOT create parent directories.
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

// Initialize applies the embedded schema to the database, then checks
// for schema drift: if the DB was created from a different schema
// version than the binary embeds, it returns [ErrSchemaDrift].
//
// Schema application itself is idempotent (every CREATE uses
// IF NOT EXISTS), but that is also the catch — adding a UNIQUE
// constraint or column to an existing table is silently a no-op,
// which is how stale DBs slip through. The user_version check is
// the safety net.
//
// Fresh DBs (user_version == 0) get the current version stamped on
// them; matching versions are a no-op; mismatches return the error.
func Initialize(db *sql.DB) error {
	slog.Debug("applying schema", "bytes", len(schemaSQL))
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	var onDisk int32
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&onDisk); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	switch {
	case onDisk == 0:
		// Fresh DB — stamp the current version. user_version doesn't
		// support parameter binding, so the value is interpolated
		// directly; schemaVersion is computed from embedded source
		// (not user input), so injection isn't a concern.
		stmt := fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion)
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
		slog.Debug("schema version stamped", "version", schemaVersion)
	case onDisk == schemaVersion:
		// Matched — nothing to do.
	default:
		return fmt.Errorf("%w: db has version %d, binary expects %d; "+
			"`orbit` currently has no migrations -- destroy and re-init the database "+
			"(`orbit destroy --yes && orbit init`)",
			ErrSchemaDrift, onDisk, schemaVersion)
	}
	return nil
}
