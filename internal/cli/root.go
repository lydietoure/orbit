package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "orbit",
	Short: "Your developer universe, mapped and in motion",
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(workCmd)
	rootCmd.AddCommand(linkCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
