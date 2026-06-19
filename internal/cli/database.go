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

// errNotInitialized is returned when orbit's home directory does not
// exist — the user must run `orbit init` before any command that
// touches the database can work.
var errNotInitialized = errors.New("orbit not initialized; run 'orbit init' first")

// openDB resolves the orbit home directory, refuses to proceed if it
// does not exist (a friendly pre-flight check so commands fail with a
// useful message instead of a confusing "no such file" from SQLite),
// opens the database, and applies the schema (idempotent).
//
// On success it returns the live *sql.DB and a closer function the
// caller should `defer`.
func openDB() (*sql.DB, func(), error) {
	home, err := config.Home()
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(home); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, errNotInitialized
		}
		return nil, nil, fmt.Errorf("stat orbit home %q: %w", home, err)
	}

	path, err := config.DatabasePath()
	if err != nil {
		return nil, nil, err
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
