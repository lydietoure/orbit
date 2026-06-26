package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lydietoure/orbit/internal/config"
	"github.com/lydietoure/orbit/internal/db"
)

// setupInitializedHome seeds an ORBIT_HOME pointing at a fresh
// tempdir with an initialized DB. ORBIT_DOCK is also cleared so
// the test starts from a known dock state regardless of the
// developer's shell environment.
func setupInitializedHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)
	t.Setenv(DockEnv, "")

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
}

// absExample returns a platform-appropriate absolute path for the
// "is absolute → used as-is" rule. On Windows the path needs a drive
// letter; on Unix a leading slash suffices.
func absExample(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return filepath.Join(t.TempDir(), "abs-pad") // tempdir is always rooted on a drive
	}
	return "/tmp/orbit-pad-test-abs"
}

func TestResolvePadPath_AbsolutePathUsedAsIs(t *testing.T) {
	setupInitializedHome(t)
	abs := absExample(t)

	// Even with a dock root configured, an absolute name wins.
	t.Setenv(DockEnv, t.TempDir())

	got, err := ResolvePadPath(context.Background(), abs, false)
	if err != nil {
		t.Fatalf("ResolvePadPath: %v", err)
	}
	if got != filepath.Clean(abs) {
		t.Errorf("got %q, want %q", got, filepath.Clean(abs))
	}
}

func TestResolvePadPath_RelativeWithDockSet_JoinsDockRoot(t *testing.T) {
	setupInitializedHome(t)
	dock := t.TempDir()
	t.Setenv(DockEnv, dock)

	got, err := ResolvePadPath(context.Background(), "myname", false)
	if err != nil {
		t.Fatalf("ResolvePadPath: %v", err)
	}
	want := filepath.Join(dock, "myname")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolvePadPath_RelativeWithDockUnset_JoinsCWD(t *testing.T) {
	setupInitializedHome(t)
	cwd := t.TempDir()
	t.Chdir(cwd)

	got, err := ResolvePadPath(context.Background(), "myname", false)
	if err != nil {
		t.Fatalf("ResolvePadPath: %v", err)
	}
	// On macOS t.TempDir() may return a /var path that resolves to a
	// /private/var symlink; filepath.Abs follows symlinks differently
	// per-platform, so compare via filepath.Join on the chdir'd value.
	want := filepath.Join(cwd, "myname")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolvePadPath_NoDockForcesCWD(t *testing.T) {
	setupInitializedHome(t)
	dock := t.TempDir()
	t.Setenv(DockEnv, dock)
	cwd := t.TempDir()
	t.Chdir(cwd)

	got, err := ResolvePadPath(context.Background(), "myname", true)
	if err != nil {
		t.Fatalf("ResolvePadPath: %v", err)
	}
	want := filepath.Join(cwd, "myname")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if strings.HasPrefix(got, dock) {
		t.Errorf("got %q unexpectedly under dock root %q despite noDock=true", got, dock)
	}
}

func TestResolvePadPath_RejectsEmptyName(t *testing.T) {
	for _, name := range []string{"", "   ", "\t\n"} {
		_, err := ResolvePadPath(context.Background(), name, false)
		if !errors.Is(err, ErrEmptyPadName) {
			t.Errorf("name %q: err = %v, want ErrEmptyPadName", name, err)
		}
	}
}

func TestProvisionPad_CreatesNewDir(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "fresh-pad")

	if err := ProvisionPad(abs); err != nil {
		t.Fatalf("ProvisionPad: %v", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatalf("stat created dir: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("created path is not a directory: %v", info.Mode())
	}
}

func TestProvisionPad_ReturnsExistedSentinelWhenDirExists(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "preexisting")
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	err := ProvisionPad(abs)
	if !errors.Is(err, ErrPadAlreadyExisted) {
		t.Errorf("err = %v, want ErrPadAlreadyExisted", err)
	}
}

func TestProvisionPad_FailsWhenPathIsAFile(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "i-am-a-file")
	if err := os.WriteFile(abs, []byte("nope"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	err := ProvisionPad(abs)
	if err == nil {
		t.Fatal("expected error for non-directory path, got nil")
	}
	if errors.Is(err, ErrPadAlreadyExisted) {
		t.Errorf("non-directory path should NOT yield ErrPadAlreadyExisted, got %v", err)
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error %q should mention 'not a directory'", err)
	}
}
