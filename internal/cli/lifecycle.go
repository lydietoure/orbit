package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lydietoure/orbit/internal/app"
	"github.com/lydietoure/orbit/internal/config"
	"github.com/lydietoure/orbit/internal/db"
	"github.com/spf13/cobra"
)

// Filesystem modes for artifacts created by `orbit init`. Octal
// notation (0o...): each digit is rwx for owner / group / other.
const (
	// homeDirPerm = rwxr-xr-x: owner can read/write/cd; everyone else
	// can read and cd into ~/.orbit but not modify it.
	homeDirPerm fs.FileMode = 0o755

	// configFilePerm = rw-r--r--: owner can read/write the YAML config;
	// everyone else can read it. No execute (it isn't a script).
	configFilePerm fs.FileMode = 0o644
)

// Flags bound to the lifecycle commands.
var (
	flagInitDryRun    bool
	flagDestroyYes    bool
	flagDestroyDryRun bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the orbit (create the orbit home, config, and database)",
	Long: `Initialize orbit by creating the orbit home directory, a default
config file, and the SQLite database. Safe to re-run: existing files are
never overwritten, and any missing pieces are recreated.

With --dry-run, prints what would happen without making any changes.`,
	Args: cobra.NoArgs,
	RunE: initializeApplication,
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove orbit's config and database",
	Long: `Delete orbit's own files (config and database). Prompts for
confirmation by default; pass --yes to skip the prompt or --dry-run to
preview without deleting.

Only orbit-managed files are removed. User data — including pads — is
never touched, even if it lives inside the orbit home directory. The
home directory itself is removed only when nothing else remains in it.`,
	Args: cobra.NoArgs,
	RunE: destroyApplication,
}

func init() {
	initCmd.Flags().BoolVarP(&flagInitDryRun, "dry-run", "n", false,
		"Print what would happen without making any changes")

	destroyCmd.Flags().BoolVarP(&flagDestroyYes, "yes", "y", false,
		"Skip the confirmation prompt")
	destroyCmd.Flags().BoolVarP(&flagDestroyDryRun, "dry-run", "n", false,
		"Print what would be deleted without removing anything")
}

func initializeApplication(cmd *cobra.Command, _ []string) error {
	home, err := config.Home()
	if err != nil {
		return fmt.Errorf("resolve orbit home: %w", err)
	}
	configPath, err := config.ConfigPath()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	dbPath, err := config.DatabasePath()
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}

	// Snapshot what already exists *before* we touch anything, so we can
	// report whether this run was a fresh init, a repair, or a no-op.
	homeExisted := pathExists(home)
	configExisted := pathExists(configPath)
	dbExisted := pathExists(dbPath)

	out := cmd.OutOrStdout()

	if flagInitDryRun {
		switch {
		case homeExisted && configExisted && dbExisted:
			fmt.Fprintf(out, "orbit already initialized at %s\n", home)
		case homeExisted:
			fmt.Fprintf(out, "Would repair orbit at %s\n", home)
			printInitDryRunDetail(out, "config", configPath, configExisted)
			printInitDryRunDetail(out, "database", dbPath, dbExisted)
		default:
			fmt.Fprintf(out, "Would initialize orbit at %s\n", home)
			printInitDryRunDetail(out, "config", configPath, configExisted)
			printInitDryRunDetail(out, "database", dbPath, dbExisted)
		}
		return nil
	}

	if err := os.MkdirAll(home, homeDirPerm); err != nil {
		return fmt.Errorf("create orbit home %q: %w", home, err)
	}
	if homeExisted {
		slog.Info("orbit home found", "path", home)
	} else {
		slog.Info("orbit home created", "path", home)
	}

	if !configExisted {
		if err := os.WriteFile(configPath, config.Default(), configFilePerm); err != nil {
			return fmt.Errorf("write default config %q: %w", configPath, err)
		}
		slog.Info("config created", "path", configPath)
	} else {
		slog.Info("config found", "path", configPath)
	}

	sqldb, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer sqldb.Close()
	if err := db.Migrate(sqldb); err != nil {
		return err
	}
	if dbExisted {
		slog.Info("database found", "path", dbPath)
	} else {
		slog.Info("database created", "path", dbPath)
	}

	switch {
	case homeExisted && configExisted && dbExisted:
		fmt.Fprintf(out, "orbit already initialized at %s\n", home)
	case homeExisted:
		fmt.Fprintf(out, "Repaired orbit at %s\n", home)
		fmt.Fprintf(out, "  config:    %s\n", configPath)
		fmt.Fprintf(out, "  database:  %s\n", dbPath)
	default:
		fmt.Fprintf(out, "Initialized orbit at %s\n", home)
		fmt.Fprintf(out, "  config:    %s\n", configPath)
		fmt.Fprintf(out, "  database:  %s\n", dbPath)
	}
	return nil
}

func destroyApplication(cmd *cobra.Command, _ []string) error {
	home, err := config.Home()
	if err != nil {
		return fmt.Errorf("resolve orbit home: %w", err)
	}
	configPath, err := config.ConfigPath()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	dbPath, err := config.DatabasePath()
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}

	out := cmd.OutOrStdout()

	if !pathExists(home) {
		fmt.Fprintf(out, "orbit not initialized (nothing to destroy at %s)\n", home)
		return nil
	}

	// Capture sizes before any deletion so all three modes (dry-run,
	// confirm prompt, success message) can report the same numbers.
	configSize := fileSize(configPath)
	dbSize := fileSize(dbPath)

	// Resolve the dock before we touch anything so we can warn when the
	// user keeps their pads inside the orbit home. The dock root may be
	// stored in the database, which destroy is about to delete.
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	dockWarning := dockInHomeWarning(ctx, home)

	if flagDestroyDryRun {
		fmt.Fprintf(out, "Would destroy orbit at %s\n", home)
		fmt.Fprintf(out, "  config:    %s (%s)\n", configPath, humanSize(configSize))
		fmt.Fprintf(out, "  database:  %s (%s)\n", dbPath, humanSize(dbSize))
		if dockWarning != "" {
			fmt.Fprint(out, dockWarning)
		}
		return nil
	}

	if !flagDestroyYes {
		fmt.Fprintf(out, "About to destroy orbit at %s\n", home)
		fmt.Fprintf(out, "  config:    %s (%s)\n", configPath, humanSize(configSize))
		fmt.Fprintf(out, "  database:  %s (%s)\n", dbPath, humanSize(dbSize))
		if dockWarning != "" {
			fmt.Fprint(out, dockWarning)
		}
		if !confirmYes(cmd.InOrStdin(), out, "Continue? [y/N]: ") {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	// Remove only the files orbit itself manages — never arbitrary user
	// data that happens to live in the home directory.
	for _, p := range orbitArtifacts(configPath, dbPath) {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove orbit file %q: %w", p, err)
		}
	}
	slog.Info("orbit files removed", "home", home)

	// Tidy up the home directory, but only if orbit's files were the
	// last thing in it. Any leftover entries are user data we must keep.
	homeRemoved, err := removeDirIfEmpty(home)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Destroyed orbit at %s\n", home)
	fmt.Fprintf(out, "  config:    deleted (%s)\n", humanSize(configSize))
	fmt.Fprintf(out, "  database:  deleted (%s)\n", humanSize(dbSize))
	if !homeRemoved {
		fmt.Fprintf(out, "  home:      preserved (%s contains other files)\n", home)
	}
	return nil
}

// orbitArtifacts returns the absolute paths of the files orbit itself
// creates inside the home directory: the config and the SQLite
// database, including the WAL/SHM sidecars SQLite writes alongside it.
// These are the only files `orbit destroy` removes.
func orbitArtifacts(configPath, dbPath string) []string {
	return []string{
		configPath,
		dbPath,
		dbPath + "-wal",
		dbPath + "-shm",
	}
}

// removeDirIfEmpty removes dir only when it has no remaining entries,
// returning whether it was removed. A non-empty directory is left in
// place because its contents are user data orbit did not create.
func removeDirIfEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("read orbit home %q: %w", dir, err)
	}
	if len(entries) > 0 {
		slog.Info("orbit home preserved (contains user data)", "path", dir, "entries", len(entries))
		return false, nil
	}
	if err := os.Remove(dir); err != nil {
		return false, fmt.Errorf("remove orbit home %q: %w", dir, err)
	}
	slog.Info("orbit home removed", "path", dir)
	return true, nil
}

// dockInHomeWarning returns a human-readable warning when the resolved
// dock root (where pads live) is inside the orbit home directory, or
// the empty string otherwise. Resolution failures are treated as "no
// warning" — destroy must not fail just because the dock is
// unreadable. The warning reassures the user that their pads are
// preserved and discourages keeping them under the orbit home.
func dockInHomeWarning(ctx context.Context, home string) string {
	root, source, err := app.GetDockRoot(ctx)
	if err != nil || source == app.DockRootUnset {
		return ""
	}
	if !pathWithin(home, root) {
		return ""
	}
	return fmt.Sprintf(
		"warning: your dock is inside the orbit home (%s)\n"+
			"  pads there are preserved, but keep them outside %s to avoid confusion\n",
		root, home)
}

// pathWithin reports whether child is parent itself or nested beneath
// it. Both paths are expected to be absolute and cleaned.
func pathWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func printInitDryRunDetail(out io.Writer, label, path string, exists bool) {
	verdict := "would create"
	if exists {
		verdict = "found"
	}
	fmt.Fprintf(out, "  %-9s %s (%s)\n", label+":", path, verdict)
}

// pathExists reports whether a filesystem entry exists at path. Errors
// other than fs.ErrNotExist are treated as "exists" so callers do not
// silently overwrite something they cannot stat.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return !errors.Is(err, fs.ErrNotExist)
}

// fileSize returns the size in bytes, or 0 if the file does not exist
// or cannot be stat'd.
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// humanSize formats a byte count as e.g. "771 B" or "49.0 KiB".
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// confirmYes prompts on out, reads a single line from in, and returns
// true only on "y" or "yes" (case-insensitive). Empty input or anything
// else returns false.
func confirmYes(in io.Reader, out io.Writer, prompt string) bool {
	fmt.Fprint(out, prompt)
	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}
