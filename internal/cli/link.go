package cli

import "github.com/spf13/cobra"

var linkCmd = &cobra.Command{
	Use:   "link [id]",
	Short: "Link an artifact to a work entry",
	Args:  cobra.MaximumNArgs(1),
}
