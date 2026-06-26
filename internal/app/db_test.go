package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lydietoure/orbit/internal/config"
	"github.com/lydietoure/orbit/internal/db"
)

// TestOpen_FailsWhenHomeMissing covers the case where ORBIT_HOME
// points at a directory that does not exist — the user has never
// run `orbit init`.
func TestOpen_FailsWhenHomeMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "definitely-not-here")
	t.Setenv(config.HomeEnv, missing)

	_, _, err := open()
	if err == nil {
		t.Fatal("expected error for missing home, got nil")
	}
	if !errors.Is(err, ErrNotInitialized) {
		t.Errorf("err = %v, want ErrNotInitialized", err)
	}
	if !strings.Contains(err.Error(), "orbit init") {
		t.Errorf("error message %q should mention `orbit init`", err)
	}
}

// TestOpen_FailsWhenDBMissing is the regression test for the bug
// where an empty ORBIT_HOME (dir exists, DB file does not) was
// treated as initialized — SQLite would silently create a fresh DB,
// db.Initialize would apply the schema, and read/write commands
// would appear to succeed without the user ever having run init.
// open() must refuse to bootstrap; that's `orbit init`'s job.
func TestOpen_FailsWhenDBMissing(t *testing.T) {
	home := t.TempDir() // dir exists, but no DB file inside
	t.Setenv(config.HomeEnv, home)

	_, _, err := open()
	if err == nil {
		t.Fatal("expected error for missing DB file, got nil")
	}
	if !errors.Is(err, ErrNotInitialized) {
		t.Errorf("err = %v, want ErrNotInitialized", err)
	}
}

// TestOpen_SucceedsWhenInitialized confirms the happy path: once
// the DB file exists (as it would after `orbit init`), open()
// returns a working handle with the schema applied.
func TestOpen_SucceedsWhenInitialized(t *testing.T) {
	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)

	// Mimic `orbit init`: create the DB file at the expected path so
	// open() sees the init marker. db.Open + db.Migrate is exactly
	// what initializeApplication does for the DB step.
	dbPath, err := config.DatabasePath()
	if err != nil {
		t.Fatalf("DatabasePath: %v", err)
	}
	seed, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("seed Open: %v", err)
	}
	if err := db.Migrate(seed); err != nil {
		t.Fatalf("seed Migrate: %v", err)
	}
	_ = seed.Close()

	d, closer, err := open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer closer()

	if err := d.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// Schema should still be applied; the singleton state row is the
	// easiest thing to assert on.
	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM state`).Scan(&count); err != nil {
		t.Fatalf("query state: %v", err)
	}
	if count != 1 {
		t.Errorf("state rows = %d, want 1", count)
	}
}
