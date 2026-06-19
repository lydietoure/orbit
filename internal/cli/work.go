package cli

import (
	"fmt"

	"github.com/lydietoure/orbit/internal/core"
	"github.com/lydietoure/orbit/internal/db"
	"github.com/spf13/cobra"
)

// newWorkCmd builds the `orbit work` parent command and its subcommands.
// Each subcommand is built by its own constructor so that flags, RunE
// closures, and the variables they bind to all live in one function —
// no package-level flag globals, no init()-side-effect coupling between
// files.
func getCmdWork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Manage work entries",
	}
	cmd.AddCommand(
		newWorkNewCmd(),
		newWorkListCmd(),
		newWorkShowCmd(),
		newWorkDeleteCmd(),
		newWorkSelectCmd(),
		newWorkForgetCmd(),
	)
	return cmd
}

//region work new
func newWorkNewCmd() *cobra.Command {
	var (
		description string
		scratchpad  string
	)
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new work entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, closer, err := openDB()
			if err != nil {
				return err
			}
			defer closer()

			entry, err := core.NewWorkEntry(core.NewWorkEntryParams{
				Title:          args[0],
				Description:    description,
				ScratchpadPath: scratchpad,
			})
			if err != nil {
				return err
			}
			if err := db.InsertWorkEntry(cmd.Context(), d, entry); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s: %q\n", entry.ID, entry.Title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "",
		"Longer explanation of this work")
	cmd.Flags().StringVarP(&scratchpad, "scratchpad", "s", "",
		"Path to a folder for experimental/scratch work on this entry")
	return cmd
}

//endregion

func newWorkListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List work entries",
	}
}

func newWorkShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [id]",
		Short: "Show details of a work entry",
		Args:  cobra.ExactArgs(1),
	}
}

func newWorkDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a work entry",
		Args:  cobra.ExactArgs(1),
	}
}

func newWorkSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select [id]",
		Short: "Select a work entry as the current focus",
		Args:  cobra.ExactArgs(1),
	}
}

func newWorkForgetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "forget",
		Short: "Clear the currently selected work entry",
	}
}
