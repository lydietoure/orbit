package db

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Open opens (or creates) the orbit database at the given path.
func Open(path string) (*sql.DB, error) {
	return nil, nil
}

// Initialize applies the schema to the database. Idempotent.
func Initialize(db *sql.DB) error {
	return nil
}
