package cli

import "github.com/spf13/cobra"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize orbit",
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Quick overview of active work entries and selected entry",
}
