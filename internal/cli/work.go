package cli

import (
	"fmt"
	"io"

	"github.com/lydietoure/orbit/internal/app"
	"github.com/lydietoure/orbit/internal/core"
	"github.com/spf13/cobra"
)

// getCmdWork builds the `orbit work` parent command and its subcommands.
// Each subcommand is built by its own constructor so that flags, RunE
// closures, and the variables they bind to all live in one function —
// no package-level flag globals, no init()-side-effect coupling.
//
// RunEs in this file are intentionally tiny: parse args, call ONE
// app function, format the result. All DB lifecycle and
// orchestration live in the app package; cli is just I/O glue.
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

// region work new
func newWorkNewCmd() *cobra.Command {
	var (
		description string
		scratchpad  string
		noSelect    bool
	)
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new work entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := app.CreateWork(cmd.Context(), app.CreateWorkParams{
				Title:          args[0],
				Description:    description,
				ScratchpadPath: scratchpad,
				NoSelect:       noSelect,
			})
			if err != nil {
				return err
			}
			verb := "Created and selected"
			if noSelect {
				verb = "Created"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %q\n", verb, entry.ID, entry.Title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "",
		"Longer explanation of this work")
	cmd.Flags().StringVarP(&scratchpad, "scratchpad", "s", "",
		"Path to a folder for experimental/scratch work on this entry")
	cmd.Flags().BoolVar(&noSelect, "no-select", false,
		"Don't auto-select the new entry (useful in scripts)")
	return cmd
}

//endregion

// region work list
func newWorkListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List work entries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := app.ListWork(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(entries) == 0 {
				fmt.Fprintln(out, "No work entries yet. Create one with `orbit work new <title>`.")
				return nil
			}
			for _, e := range entries {
				// Compact one-line format: <id>  <status>  <title>
				// IDs are fixed-width (5 chars); statuses vary so we
				// pad to the longest enum value ("in-progress" = 11).
				fmt.Fprintf(out, "%s  %-11s  %s\n", e.ID, e.Status, e.Title)
			}
			return nil
		},
	}
}

//endregion

// region work show
func newWorkShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of a work entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := app.ShowWork(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printWorkEntry(cmd.OutOrStdout(), entry)
			return nil
		},
	}
}

// printWorkEntry writes a human-readable detail view of e to w.
// Empty optional fields are rendered as "(none)" so the layout stays
// aligned and the absence is obvious.
func printWorkEntry(w io.Writer, e core.WorkEntry) {
	const timeFmt = "2006-01-02 15:04:05 MST"
	fmt.Fprintf(w, "ID:           %s\n", e.ID)
	fmt.Fprintf(w, "Title:        %s\n", e.Title)
	fmt.Fprintf(w, "Status:       %s\n", e.Status)
	if e.StatusReason != "" {
		fmt.Fprintf(w, "Reason:       %s\n", e.StatusReason)
	}
	fmt.Fprintf(w, "Description:  %s\n", orNone(e.Description))
	fmt.Fprintf(w, "Scratchpad:   %s\n", orNone(e.ScratchpadPath))
	fmt.Fprintf(w, "Created:      %s\n", e.CreatedAt.UTC().Format(timeFmt))
	fmt.Fprintf(w, "Updated:      %s\n", e.UpdatedAt.UTC().Format(timeFmt))
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

//endregion

func newWorkDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a work entry",
		Args:  cobra.ExactArgs(1),
	}
}

func newWorkSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <id>",
		Short: "Select a work entry as the current focus",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := app.SelectWork(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Selected %s: %q\n", entry.ID, entry.Title)
			return nil
		},
	}
}

func newWorkForgetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "forget",
		Short: "Clear the currently selected work entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := app.ForgetSelectedWork(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Cleared selected work entry.")
			return nil
		},
	}
}
