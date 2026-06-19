package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/lydietoure/orbit/internal/config"
	"github.com/lydietoure/orbit/internal/db"
)

// errNotInitialized is returned when orbit has not been initialized —
// concretely, when the SQLite database file is missing. The user must
// run `orbit init` first. The DB file is the init marker (rather than
// the orbit home dir) because SQLite will silently create a fresh DB
// on first open, so a bare directory check would let read/write
// commands quietly bootstrap themselves and look like they succeeded.
var errNotInitialized = errors.New("orbit not initialized; run 'orbit init' first")

// openDB is the entry point for every command that needs a live DB
// handle EXCEPT `orbit init` itself (which has its own create-or-open
// path). It refuses to proceed if orbit has not been initialized
// (DB file missing), opens the database, applies the schema
// (idempotent — covers schema upgrades on existing DBs), and returns
// the live *sql.DB plus a closer the caller should `defer`.
func openDB() (*sql.DB, func(), error) {
	path, err := config.DatabasePath()
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, errNotInitialized
		}
		return nil, nil, fmt.Errorf("stat orbit database %q: %w", path, err)
	}

	d, err := db.Open(path)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Initialize(d); err != nil {
		_ = d.Close()
		return nil, nil, err
	}
	return d, func() { _ = d.Close() }, nil
}
