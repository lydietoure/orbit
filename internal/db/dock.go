package db

// This file is the storage gateway for the dock columns on the
// `state` singleton: the configured pad-root path and the
// auto-create flag. Same layering as the other gateways here — no
// validation, no env-var resolution, no business rules. Callers
// hand us values that are already valid; the app layer is where
// env-var overrides and path absolutization live.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNoDockRoot is returned by [GetDockRoot] when the state
// singleton has no dock root configured (the default state, and the
// state after [UnsetDockRoot]). Detect with errors.Is.
//
// Distinguishing "unset" from "set to empty string" lets the app
// layer fall back to the env var only when the user has not made an
// explicit choice. The DB column is TEXT NULL exactly for this.
var ErrNoDockRoot = errors.New("no dock root is configured")

// GetDockRoot returns the dock root stored in the state singleton,
// or [ErrNoDockRoot] if none is set. Returns the raw stored value;
// the caller is responsible for any env-var override or
// absolutization.
func GetDockRoot(ctx context.Context, db *sql.DB) (string, error) {
	var root sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT dock_root FROM state WHERE id = 1`).Scan(&root)
	if errors.Is(err, sql.ErrNoRows) {
		// The schema seeds the singleton at apply time, so this
		// branch only fires on a corrupted DB. Treat it as "unset".
		return "", ErrNoDockRoot
	}
	if err != nil {
		return "", fmt.Errorf("get dock root: %w", err)
	}
	if !root.Valid {
		return "", ErrNoDockRoot
	}
	return root.String, nil
}

// SetDockRoot stores path as the dock root. The caller is
// responsible for any validation (non-empty, absolute, etc.);
// passing "" here is treated as "unset" via [UnsetDockRoot] for
// safety, since "" is indistinguishable from NULL at the SQL level
// for our purposes.
func SetDockRoot(ctx context.Context, db *sql.DB, path string) error {
	if path == "" {
		return UnsetDockRoot(ctx, db)
	}
	_, err := db.ExecContext(ctx,
		`UPDATE state SET dock_root = ? WHERE id = 1`, path)
	if err != nil {
		return fmt.Errorf("set dock root: %w", err)
	}
	return nil
}

// UnsetDockRoot clears the dock root. Safe to call when nothing is
// configured — the UPDATE is a no-op in that case.
func UnsetDockRoot(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx,
		`UPDATE state SET dock_root = NULL WHERE id = 1`)
	if err != nil {
		return fmt.Errorf("unset dock root: %w", err)
	}
	return nil
}

// GetDockAutoCreate returns whether pad auto-creation is
// enabled. Defaults to false on a fresh install.
func GetDockAutoCreate(ctx context.Context, db *sql.DB) (bool, error) {
	var v int
	err := db.QueryRowContext(ctx,
		`SELECT dock_auto_create FROM state WHERE id = 1`).Scan(&v)
	if err != nil {
		return false, fmt.Errorf("get dock auto-create: %w", err)
	}
	return v != 0, nil
}

// SetDockAutoCreate stores the auto-create flag. SQLite has no
// native bool, so we map true→1 / false→0 explicitly; the column's
// CHECK constraint rejects any other value.
func SetDockAutoCreate(ctx context.Context, db *sql.DB, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := db.ExecContext(ctx,
		`UPDATE state SET dock_auto_create = ? WHERE id = 1`, v)
	if err != nil {
		return fmt.Errorf("set dock auto-create: %w", err)
	}
	return nil
}
