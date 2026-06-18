package cli

import "github.com/spf13/cobra"

var workCmd = &cobra.Command{
	Use:   "work",
	Short: "Manage work entries",
}

func init() {
	workCmd.AddCommand(workNewCmd)
	workCmd.AddCommand(workListCmd)
	workCmd.AddCommand(workShowCmd)
	workCmd.AddCommand(workDeleteCmd)
	workCmd.AddCommand(workSelectCmd)
	workCmd.AddCommand(workForgetCmd)
}

var workNewCmd = &cobra.Command{
	Use:   "new [title]",
	Short: "Create a new work entry",
	Args:  cobra.ExactArgs(1),
}

var workListCmd = &cobra.Command{
	Use:   "list",
	Short: "List work entries",
}

var workShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show details of a work entry",
	Args:  cobra.MaximumNArgs(1),
}

var workDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a work entry",
	Args:  cobra.ExactArgs(1),
}

var workSelectCmd = &cobra.Command{
	Use:   "select [id]",
	Short: "Select a work entry as the current focus",
	Args:  cobra.ExactArgs(1),
}

var workForgetCmd = &cobra.Command{
	Use:   "forget",
	Short: "Clear the currently selected work entry",
}
