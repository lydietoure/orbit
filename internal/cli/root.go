package cli

import (
	"os"
	"strings"

	"github.com/lydietoure/orbit/internal/diag"
	"github.com/lydietoure/orbit/internal/version"
	"github.com/spf13/cobra"
)

// Environment variables read at startup. Explicit CLI flags always win
// over their env-var counterparts.
const (
	verboseEnv = "ORBIT_VERBOSE"
	debugEnv   = "ORBIT_DEBUG"
)

var (
	flagVerbose bool
	flagDebug   bool
)

var rootCmd = &cobra.Command{
	Use:   "orbit",
	Short: "Your developer universe, mapped and in motion",
	// PersistentPreRunE on the root runs before every subcommand. If any
	// subcommand ever defines its own PersistentPreRunE, it must call
	// diag.Setup itself (cobra does NOT chain inherited PreRuns).
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Explicit flag wins; otherwise fall back to env var.
		verbose := flagVerbose || envBool(verboseEnv)
		debug := flagDebug || envBool(debugEnv)
		diag.Setup(verbose, debug)
		return nil
	},
}

func init() {
	rootCmd.Version = version.String()

	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false,
		"Enable verbose (info-level) logging (or set ORBIT_VERBOSE=1)")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false,
		"Enable debug logging, implies --verbose (or set ORBIT_DEBUG=1)")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(getCmdWork())
	rootCmd.AddCommand(linkCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// envBool reports whether the named env var is set to a truthy value.
// Truthy: "1", "true", "yes", "on" (case-insensitive). Empty / unset /
// any other value is false.
func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
