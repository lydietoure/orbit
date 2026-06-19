package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lydietoure/orbit/internal/config"
)

// TestOpenDB_FailsIfHomeMissing verifies the pre-flight check: pointing
// ORBIT_HOME at a directory that does not exist should produce a
// friendly "not initialized" error rather than a confusing DB error.
func TestOpenDB_FailsIfHomeMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "definitely-not-here")
	t.Setenv(config.HomeEnv, missing)

	_, _, err := openDB()
	if err == nil {
		t.Fatal("expected error for missing home, got nil")
	}
	if !errors.Is(err, errNotInitialized) {
		t.Errorf("err = %v, want errNotInitialized", err)
	}
	if !strings.Contains(err.Error(), "orbit init") {
		t.Errorf("error message %q should mention `orbit init`", err)
	}
}

// TestOpenDB_OpensWhenHomeExists confirms the happy path: an existing
// (empty) home dir yields a working DB handle with the schema applied.
func TestOpenDB_OpensWhenHomeExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)

	d, closer, err := openDB()
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer closer()

	if err := d.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	// Schema should be applied (Initialize ran), so the singleton
	// state row must be present.
	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM state`).Scan(&count); err != nil {
		t.Fatalf("query state: %v", err)
	}
	if count != 1 {
		t.Errorf("state rows = %d, want 1", count)
	}
}
