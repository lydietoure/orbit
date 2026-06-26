// Package app is Orbit's use-case layer — it sits between the cli
// (flag parsing + printing) and the data layers (core + db).
//
// Layering:
//
//	core/ — domain types and validation, pure Go.
//	db/   — per-table SQL gateways, dumb storage.
//	app/  — opens the database, calls core+db in the right order,
//	        returns results. THIS PACKAGE.
//	cli/  — parses flags, calls one app function, prints the result.
//
// A use-case function looks like:
//
//	func CreateWork(ctx, params) (core.WorkEntry, error) {
//	    d, closer, err := open() // <-- I/O lifecycle owned here
//	    if err != nil { return ... }
//	    defer closer()
//	    entry, err := core.NewWorkEntry(...)
//	    if err != nil { return ... }
//	    if err := db.InsertWorkEntry(ctx, d, entry); err != nil {
//	        return ...
//	    }
//	    return entry, nil
//	}
//
// The cli's RunE then becomes ~6 lines: parse args, call CreateWork,
// print. No *sql.DB in cli, no defer-close in cli, no orchestration
// in cli.
package app

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/lydietoure/orbit/internal/config"
	"github.com/lydietoure/orbit/internal/db"
)

// ErrNotInitialized is returned by the use-case functions when orbit
// has not been initialized — concretely, when the SQLite database
// file is missing. The user must run `orbit init` first.
//
// The DB file (not the home directory) is the init marker because
// SQLite silently creates a fresh DB on first open, so a bare
// directory check would let read/write commands quietly bootstrap
// themselves and look like they succeeded.
var ErrNotInitialized = errors.New("orbit not initialized; run 'orbit init' first")

// open is the package-internal entry point for every use case that
// needs a live DB handle. It refuses to proceed if orbit has not
// been initialized (DB file missing), opens the database, applies
// the schema (idempotent — also runs the drift check), and returns
// the live *sql.DB plus a closer the caller should defer.
//
// `orbit init` does NOT go through this helper; it has its own
// create-or-open path in cli/lifecycle.go because its job is to
// create the DB in the first place.
//
// open calls [db.Migrate] rather than [db.Initialize], so it also
// applies any pending schema migrations on every invocation.
func open() (*sql.DB, func(), error) {
	path, err := config.DatabasePath()
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, ErrNotInitialized
		}
		return nil, nil, fmt.Errorf("stat orbit database %q: %w", path, err)
	}

	d, err := db.Open(path)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Migrate(d); err != nil {
		_ = d.Close()
		return nil, nil, err
	}
	return d, func() { _ = d.Close() }, nil
}
