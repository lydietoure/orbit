package cli

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lydietoure/orbit/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	// Silence slog output across all tests in this package.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// runInitFor drives runInit against an isolated ORBIT_HOME and returns
// the stdout it produced.
func runInitFor(t *testing.T, home string) string {
	t.Helper()
	t.Setenv(config.HomeEnv, home)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := initializeApplication(cmd, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	return buf.String()
}

func TestInit_FreshHome_CreatesEverything(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit") // does NOT exist yet
	out := runInitFor(t, home)

	if !strings.HasPrefix(out, "Initialized orbit at ") {
		t.Errorf("output should start with 'Initialized orbit at ', got:\n%s", out)
	}

	for _, name := range []string{config.ConfigFileName, config.DatabaseFileName} {
		p := filepath.Join(home, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist after init, got: %v", p, err)
		}
	}
}

func TestInit_AlreadyInitialized_IsNoop(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit")

	_ = runInitFor(t, home) // first run

	configPath := filepath.Join(home, config.ConfigFileName)
	want := []byte("user-edited content\n")
	if err := os.WriteFile(configPath, want, 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	out := runInitFor(t, home) // second run
	if !strings.HasPrefix(out, "orbit already initialized at ") {
		t.Errorf("expected 'orbit already initialized at ...', got:\n%s", out)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("init overwrote user-edited config:\nwant %q\n got %q", want, got)
	}
}

func TestInit_HomeExistsButDBMissing_Repairs(t *testing.T) {
	home := t.TempDir() // exists but empty
	out := runInitFor(t, home)

	if !strings.HasPrefix(out, "Repaired orbit at ") {
		t.Errorf("expected 'Repaired orbit at ...', got:\n%s", out)
	}

	for _, name := range []string{config.ConfigFileName, config.DatabaseFileName} {
		p := filepath.Join(home, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist after repair, got: %v", p, err)
		}
	}
}

func TestInit_RejectsPositionalArgs(t *testing.T) {
	// initCmd.Args == cobra.NoArgs; verify it rejects extras.
	if err := initCmd.Args(initCmd, []string{"unexpected"}); err == nil {
		t.Error("initCmd.Args should reject positional arguments")
	}
}

// runInitDryRun drives initializeApplication with --dry-run set.
func runInitDryRun(t *testing.T, home string) string {
	t.Helper()
	t.Setenv(config.HomeEnv, home)

	prev := flagInitDryRun
	flagInitDryRun = true
	t.Cleanup(func() { flagInitDryRun = prev })

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := initializeApplication(cmd, nil); err != nil {
		t.Fatalf("init --dry-run: %v", err)
	}
	return buf.String()
}

func TestInit_DryRun_FreshHome_DoesNotCreate(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit") // does not exist
	out := runInitDryRun(t, home)

	if !strings.HasPrefix(out, "Would initialize orbit at ") {
		t.Errorf("expected 'Would initialize orbit at ...', got:\n%s", out)
	}
	if !strings.Contains(out, "would create") {
		t.Errorf("expected 'would create' markers, got:\n%s", out)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create %s, but stat err = %v", home, err)
	}
}

func TestInit_DryRun_AlreadyInitialized(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit")
	_ = runInitFor(t, home) // first wet run

	out := runInitDryRun(t, home)
	if !strings.HasPrefix(out, "orbit already initialized at ") {
		t.Errorf("expected 'orbit already initialized at ...', got:\n%s", out)
	}
}

func TestInit_DryRun_Repair_DoesNotCreate(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit")
	_ = runInitFor(t, home) // create everything

	dbPath := filepath.Join(home, config.DatabaseFileName)
	if err := os.Remove(dbPath); err != nil {
		t.Fatalf("remove db: %v", err)
	}

	out := runInitDryRun(t, home)
	if !strings.HasPrefix(out, "Would repair orbit at ") {
		t.Errorf("expected 'Would repair orbit at ...', got:\n%s", out)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("dry-run should not recreate db, but stat err = %v", err)
	}
}

// destroyOpts captures the flags + stdin for a destroy run.
type destroyOpts struct {
	yes    bool
	dryRun bool
	stdin  string
}

func runDestroy(t *testing.T, home string, opts destroyOpts) string {
	t.Helper()
	t.Setenv(config.HomeEnv, home)

	prevYes, prevDry := flagDestroyYes, flagDestroyDryRun
	flagDestroyYes = opts.yes
	flagDestroyDryRun = opts.dryRun
	t.Cleanup(func() {
		flagDestroyYes = prevYes
		flagDestroyDryRun = prevDry
	})

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(strings.NewReader(opts.stdin))

	if err := destroyApplication(cmd, nil); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	return buf.String()
}

func TestDestroy_NothingToDestroy(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit") // never created
	out := runDestroy(t, home, destroyOpts{yes: true})

	if !strings.HasPrefix(out, "orbit not initialized") {
		t.Errorf("expected 'orbit not initialized ...', got:\n%s", out)
	}
}

func TestDestroy_DryRun_PreservesFiles(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit")
	_ = runInitFor(t, home)

	out := runDestroy(t, home, destroyOpts{dryRun: true})
	if !strings.HasPrefix(out, "Would destroy orbit at ") {
		t.Errorf("expected 'Would destroy orbit at ...', got:\n%s", out)
	}

	for _, name := range []string{config.ConfigFileName, config.DatabaseFileName} {
		p := filepath.Join(home, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("dry-run should not delete %s, got: %v", p, err)
		}
	}
}

func TestDestroy_YesFlag_DeletesEverything(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit")
	_ = runInitFor(t, home)

	out := runDestroy(t, home, destroyOpts{yes: true})
	if !strings.HasPrefix(out, "Destroyed orbit at ") {
		t.Errorf("expected 'Destroyed orbit at ...', got:\n%s", out)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("home should be gone, stat err = %v", err)
	}
}

func TestDestroy_PromptAccept(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit")
	_ = runInitFor(t, home)

	out := runDestroy(t, home, destroyOpts{stdin: "y\n"})
	if !strings.Contains(out, "Destroyed orbit at ") {
		t.Errorf("expected 'Destroyed orbit at ...' after y, got:\n%s", out)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("home should be gone after y, stat err = %v", err)
	}
}

func TestDestroy_PromptReject_PreservesFiles(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit")
	_ = runInitFor(t, home)

	out := runDestroy(t, home, destroyOpts{stdin: "n\n"})
	if !strings.Contains(out, "aborted") {
		t.Errorf("expected 'aborted' after n, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(home, config.DatabaseFileName)); err != nil {
		t.Errorf("db should still exist after rejection, got: %v", err)
	}
}

func TestDestroy_PromptEmpty_PreservesFiles(t *testing.T) {
	home := filepath.Join(t.TempDir(), "orbit")
	_ = runInitFor(t, home)

	out := runDestroy(t, home, destroyOpts{stdin: "\n"})
	if !strings.Contains(out, "aborted") {
		t.Errorf("expected 'aborted' after empty input, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(home, config.DatabaseFileName)); err != nil {
		t.Errorf("db should still exist after empty answer, got: %v", err)
	}
}

func TestDestroy_RejectsPositionalArgs(t *testing.T) {
	if err := destroyCmd.Args(destroyCmd, []string{"unexpected"}); err == nil {
		t.Error("destroyCmd.Args should reject positional arguments")
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{771, "771 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{49 * 1024, "49.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
	}
	for _, c := range cases {
		if got := humanSize(c.n); got != c.want {
			t.Errorf("humanSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
