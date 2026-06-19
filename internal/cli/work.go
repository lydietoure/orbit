package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"

	"github.com/lydietoure/orbit/internal/db"
	"github.com/spf13/cobra"
)

var workCmd = &cobra.Command{
	Use:   "work",
	Short: "Manage work entries",
}

// Flags for `orbit work new`.
var (
	flagWorkNewDescription string
	flagWorkNewScratchpad  string
)

func init() {
	workCmd.AddCommand(workNewCmd)
	workCmd.AddCommand(workListCmd)
	workCmd.AddCommand(workShowCmd)
	workCmd.AddCommand(workDeleteCmd)
	workCmd.AddCommand(workSelectCmd)
	workCmd.AddCommand(workForgetCmd)

	workNewCmd.Flags().StringVarP(&flagWorkNewDescription, "description", "d", "",
		"Longer explanation of this work")
	workNewCmd.Flags().StringVarP(&flagWorkNewScratchpad, "scratchpad", "s", "",
		"Path to a folder for experimental/scratch work on this entry")
}

var workNewCmd = &cobra.Command{
	Use:   "new <title>",
	Short: "Create a new work entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, closer, err := openDB()
		if err != nil {
			return err
		}
		defer closer()

		return runWorkNew(cmd.Context(), d, cmd.OutOrStdout(), workNewOpts{
			title:       args[0],
			description: flagWorkNewDescription,
			scratchpad:  flagWorkNewScratchpad,
		})
	},
}

// workNewOpts groups the inputs runWorkNew needs. Kept as a small
// internal struct so tests can call the helper directly without going
// through cobra flag globals.
type workNewOpts struct {
	title       string
	description string
	scratchpad  string
}

// runWorkNew is the testable core of `orbit work new`. It creates a
// new work entry (status defaults to "new" inside db.CreateWorkEntry)
// and prints a confirmation line to out.
func runWorkNew(ctx context.Context, d *sql.DB, out io.Writer, opts workNewOpts) error {
	entry, err := db.CreateWorkEntry(ctx, d, db.CreateWorkEntryParams{
		Title:          opts.title,
		Description:    opts.description,
		ScratchpadPath: opts.scratchpad,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Created %s: %q\n", entry.ID, entry.Title)
	return nil
}

var workListCmd = &cobra.Command{
	Use:   "list",
	Short: "List work entries",
}

var workShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show details of a work entry",
	Args:  cobra.ExactArgs(1),
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
