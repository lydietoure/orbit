package app

// This file is the use-case layer for "the dock": the configured
// root directory where pads live, plus the auto-create flag
// that controls whether `work new` provisions per-entry subdirs
// under it automatically.
//
// Resolution order for the root:
//
//  1. ORBIT_DOCK env var (if set and non-empty).
//  2. The value persisted in the DB via `orbit config pad set-root`.
//  3. Unset — returned as the empty string with [DockRootUnset].
//
// Only the DB-stored value is mutable through the CLI; the env var
// is read-only and exists for transient overrides (CI jobs,
// scripted workspaces). The auto-create flag has no env override —
// it's a deliberate persistent setting.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lydietoure/orbit/internal/db"
)

// DockEnv is the environment variable that, if set and non-empty,
// overrides the persisted dock root.
const DockEnv = "ORBIT_DOCK"

// DockRootSource describes where a resolved dock root came from.
type DockRootSource int

const (
	// DockRootUnset means no dock root is configured anywhere.
	DockRootUnset DockRootSource = iota
	// DockRootFromEnv means the value came from the [DockEnv]
	// environment variable.
	DockRootFromEnv
	// DockRootFromConfig means the value came from the DB-stored
	// `orbit config pad set-root` value.
	DockRootFromConfig
)

// String renders the source for human-readable output.
func (s DockRootSource) String() string {
	switch s {
	case DockRootFromEnv:
		return "env"
	case DockRootFromConfig:
		return "config"
	default:
		return "unset"
	}
}

// ErrEmptyDockRoot is returned by [SetDockRoot] when the supplied
// path is empty or whitespace-only. Use [UnsetDockRoot] to clear.
var ErrEmptyDockRoot = errors.New("dock root path is empty; use `orbit config pad unset-root` to clear")

// GetDockRoot resolves the current dock root.
//
// Returns ("", DockRootUnset, nil) when neither the env var nor the
// DB has a value — the caller should treat that as "the user hasn't
// configured a dock yet". When the env var supplies the value it is
// absolutized so the caller sees the same thing a [SetDockRoot]
// call would have stored.
func GetDockRoot(ctx context.Context) (string, DockRootSource, error) {
	if v := os.Getenv(DockEnv); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", DockRootUnset, fmt.Errorf("resolve %s=%q: %w", DockEnv, v, err)
		}
		return abs, DockRootFromEnv, nil
	}

	d, closer, err := open()
	if err != nil {
		return "", DockRootUnset, err
	}
	defer closer()

	root, err := db.GetDockRoot(ctx, d)
	if errors.Is(err, db.ErrNoDockRoot) {
		return "", DockRootUnset, nil
	}
	if err != nil {
		return "", DockRootUnset, err
	}
	return root, DockRootFromConfig, nil
}

// SetDockRoot persists path as the dock root. The supplied path is
// absolutized so subsequent reads return a stable value regardless
// of the working directory at set time. The env var, if set, still
// takes precedence at read time — that is by design (env >
// config), and `get-root` surfaces the source.
//
// Returns the absolutized path that was stored.
func SetDockRoot(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", ErrEmptyDockRoot
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve dock root %q: %w", path, err)
	}

	d, closer, err := open()
	if err != nil {
		return "", err
	}
	defer closer()

	if err := db.SetDockRoot(ctx, d, abs); err != nil {
		return "", err
	}
	return abs, nil
}

// UnsetDockRoot clears the persisted dock root. The env var (if
// set) is unaffected — there's no portable way to unset a parent
// process's environment from a child, and that's a feature: env
// overrides are meant to be transient.
func UnsetDockRoot(ctx context.Context) error {
	d, closer, err := open()
	if err != nil {
		return err
	}
	defer closer()

	return db.UnsetDockRoot(ctx, d)
}

// GetDockAutoCreate returns whether pad auto-provisioning is
// enabled. The flag has no env override.
func GetDockAutoCreate(ctx context.Context) (bool, error) {
	d, closer, err := open()
	if err != nil {
		return false, err
	}
	defer closer()

	return db.GetDockAutoCreate(ctx, d)
}

// SetDockAutoCreate persists the auto-create flag.
func SetDockAutoCreate(ctx context.Context, enabled bool) error {
	d, closer, err := open()
	if err != nil {
		return err
	}
	defer closer()

	return db.SetDockAutoCreate(ctx, d, enabled)
}
