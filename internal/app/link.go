package app

// Use cases behind `orbit link`: attach (and detach, and list) typed
// artifacts on a work entry. Same layering as the rest of the app
// package — open the DB, resolve the target entry, validate through
// core, call the db gateways in order.
//
// Two conventions specific to linking:
//
//   - Local-path artifacts (repo, dir, file, note) are stored as
//     ABSOLUTE paths so they remain stable regardless of where the
//     user was standing when they linked. Absolutization lives here,
//     not in core, because it depends on the working directory.
//   - Orbit only references files; it never owns them. A path that does
//     not exist on disk is therefore at most a warning, returned as a
//     non-empty `warning` string the CLI surfaces on stderr — never an
//     error that blocks the link.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
)

// LinkArtifact attaches a typed artifact to a work entry. The raw
// value is validated/normalized for the type via core; local-path
// types are absolutized and their existence checked (a missing path
// yields a non-empty warning, not an error). Linking is idempotent.
//
// Returns the resolved entry id, the stored value, and an optional
// warning. An empty id falls back to the selected entry (returns
// [ErrNoTargetWorkEntry] if nothing is selected); wraps
// [db.ErrWorkEntryNotFound] when the entry is missing.
func LinkArtifact(ctx context.Context, id string, t core.ArtifactType, rawValue string) (resolvedID, value, warning string, err error) {
	if !t.Valid() {
		return "", "", "", fmt.Errorf("unknown artifact type %q", t)
	}
	value, err = t.NormalizeValue(rawValue)
	if err != nil {
		return "", "", "", err
	}

	d, closer, err := open()
	if err != nil {
		return "", "", "", err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", "", "", err
	}
	if _, err := db.GetWorkEntry(ctx, d, target); err != nil {
		return "", "", "", err
	}

	if t.IsLocalPath() {
		abs, aerr := filepath.Abs(value)
		if aerr != nil {
			return "", "", "", fmt.Errorf("resolve %s path %q: %w", t, value, aerr)
		}
		value = abs
		warning = missingPathWarning(abs)
	}

	artifact := core.Artifact{
		WorkEntryID: target,
		Type:        t,
		Value:       value,
		CreatedAt:   time.Now().UTC(),
	}
	if err := db.AddArtifact(ctx, d, artifact); err != nil {
		return "", "", "", err
	}
	return target, value, warning, nil
}

// UnlinkArtifact removes a typed artifact from a work entry. The value
// is normalized the same way [LinkArtifact] stored it (local paths are
// absolutized) so the caller can pass the same string they linked
// with. Wraps [db.ErrArtifactNotOnEntry] when the artifact is absent,
// [db.ErrWorkEntryNotFound] when the entry is missing, and
// [ErrNoTargetWorkEntry] when no id is given and nothing is selected.
func UnlinkArtifact(ctx context.Context, id string, t core.ArtifactType, rawValue string) (resolvedID, value string, err error) {
	if !t.Valid() {
		return "", "", fmt.Errorf("unknown artifact type %q", t)
	}
	value, err = t.NormalizeValue(rawValue)
	if err != nil {
		return "", "", err
	}

	d, closer, err := open()
	if err != nil {
		return "", "", err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", "", err
	}
	if _, err := db.GetWorkEntry(ctx, d, target); err != nil {
		return "", "", err
	}

	if t.IsLocalPath() {
		abs, aerr := filepath.Abs(value)
		if aerr != nil {
			return "", "", fmt.Errorf("resolve %s path %q: %w", t, value, aerr)
		}
		value = abs
	}
	if err := db.RemoveArtifact(ctx, d, target, t, value); err != nil {
		return "", "", err
	}
	return target, value, nil
}

// ListLinks returns the artifacts linked to a work entry (oldest-first).
// An empty id falls back to the selected entry; wraps
// [db.ErrWorkEntryNotFound] / [ErrNoTargetWorkEntry] as the other use
// cases do.
func ListLinks(ctx context.Context, id string) (resolvedID string, artifacts []core.Artifact, err error) {
	d, closer, err := open()
	if err != nil {
		return "", nil, err
	}
	defer closer()

	target, err := resolveTargetID(ctx, d, id)
	if err != nil {
		return "", nil, err
	}
	entry, err := db.GetWorkEntry(ctx, d, target)
	if err != nil {
		return "", nil, err
	}
	return target, entry.Artifacts, nil
}

// missingPathWarning returns a human-readable warning when abs does not
// exist on disk, and "" otherwise. Orbit only references files, so a
// missing path is informational — never a hard failure. A stat error
// other than "not found" (e.g. a permission problem) is treated as
// "can't confirm it's missing" and produces no warning.
func missingPathWarning(abs string) string {
	if _, err := os.Stat(abs); errors.Is(err, fs.ErrNotExist) {
		return fmt.Sprintf("linked path does not exist yet: %s", abs)
	}
	return ""
}
