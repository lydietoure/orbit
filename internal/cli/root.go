package cli

import (
	"github.com/lydietoure/orbit/internal/diag"
	"github.com/spf13/cobra"
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
		diag.Setup(flagVerbose, flagDebug)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose (info-level) logging")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable debug logging (implies --verbose)")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(workCmd)
	rootCmd.AddCommand(linkCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
