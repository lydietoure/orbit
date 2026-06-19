package app

// This file is the use-case layer for "pads": the per-entry folders
// where the user does experimental/scratch work for a WorkEntry. The
// dock (configured root + auto-create flag) lives in dock.go; pads
// are the things that get provisioned inside it.
//
// The two operations here are deliberately split:
//
//   - ResolvePadPath turns the user-supplied <name> into an absolute
//     path using the resolution rules from docs/DESIGN.md. No FS
//     mutation.
//   - ProvisionPad does the os.MkdirAll, distinguishing "freshly
//     created" from "already existed" so callers can warn without
//     erroring.
//
// CreateWork composes both. Future `orbit work pad <path>` will
// reuse ResolvePadPath without re-provisioning.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrEmptyPadName is returned by ResolvePadPath when the supplied
// name is empty or whitespace-only. Callers gate on PadPath != ""
// before calling, but the explicit check guards against a relative
// name that becomes empty after trimming.
var ErrEmptyPadName = errors.New("pad name is empty")

// ErrPadAlreadyExisted is returned by ProvisionPad when the target
// directory already existed before the call. It is informational,
// not a failure — callers typically log a warning and proceed.
var ErrPadAlreadyExisted = errors.New("pad path already exists")

// ResolvePadPath turns a user-supplied <name> into an absolute path
// for a per-entry pad folder. The rules (see docs/DESIGN.md):
//
//  1. If <name> is an absolute path → used as-is (cleaned).
//  2. If <name> is relative and a dock root is configured (env or
//     DB) and noDock is false → joined with the dock root.
//  3. Otherwise → joined with the current working directory.
//
// The returned path is always absolute. ResolvePadPath does not
// create any directory; provisioning is separate via ProvisionPad.
func ResolvePadPath(ctx context.Context, name string, noDock bool) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", ErrEmptyPadName
	}
	if filepath.IsAbs(name) {
		return filepath.Clean(name), nil
	}

	var base string
	if !noDock {
		root, source, err := GetDockRoot(ctx)
		if err != nil {
			return "", err
		}
		if source != DockRootUnset {
			base = root
		}
	}
	if base == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve pad path %q against CWD: %w", name, err)
		}
		base = cwd
	}
	abs, err := filepath.Abs(filepath.Join(base, name))
	if err != nil {
		return "", fmt.Errorf("resolve pad path %q: %w", name, err)
	}
	return abs, nil
}

// ProvisionPad creates the pad directory at abs if it does not
// already exist. On success the directory is guaranteed to be
// present. If the directory existed already, [ErrPadAlreadyExisted]
// is returned — non-fatal, the caller should warn and proceed. If a
// non-directory file exists at the path, a real error is returned.
func ProvisionPad(abs string) error {
	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return fmt.Errorf("create pad dir %q: %w", abs, err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("stat pad dir %q: %w", abs, err)
	case !info.IsDir():
		return fmt.Errorf("pad path %q exists but is not a directory", abs)
	default:
		return ErrPadAlreadyExisted
	}
}
