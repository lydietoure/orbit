package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"

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
	Short: "Remove the orbit home directory and all its contents",
	Long: `Delete the orbit home directory (config and database). Prompts
for confirmation by default; pass --yes to skip the prompt or --dry-run
to preview without deleting.

Pad folders are NOT touched.`,
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
	if err := db.Initialize(sqldb); err != nil {
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

	if flagDestroyDryRun {
		fmt.Fprintf(out, "Would destroy orbit at %s\n", home)
		fmt.Fprintf(out, "  config:    %s (%s)\n", configPath, humanSize(configSize))
		fmt.Fprintf(out, "  database:  %s (%s)\n", dbPath, humanSize(dbSize))
		return nil
	}

	if !flagDestroyYes {
		fmt.Fprintf(out, "About to destroy orbit at %s\n", home)
		fmt.Fprintf(out, "  config:    %s (%s)\n", configPath, humanSize(configSize))
		fmt.Fprintf(out, "  database:  %s (%s)\n", dbPath, humanSize(dbSize))
		if !confirmYes(cmd.InOrStdin(), out, "Continue? [y/N]: ") {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	if err := os.RemoveAll(home); err != nil {
		return fmt.Errorf("remove orbit home %q: %w", home, err)
	}
	slog.Info("orbit home removed", "path", home)

	fmt.Fprintf(out, "Destroyed orbit at %s\n", home)
	fmt.Fprintf(out, "  config:    deleted (%s)\n", humanSize(configSize))
	fmt.Fprintf(out, "  database:  deleted (%s)\n", humanSize(dbSize))
	return nil
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
