package cli

import (
	"fmt"
	"io"
	"strings"

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
		newWorkSelectedCmd(),
		newWorkForgetCmd(),
		newWorkTagCmd(),
	)
	return cmd
}

// region work new
func newWorkNewCmd() *cobra.Command {
	var (
		description string
		pad         string
		tags        []string
		noSelect    bool
	)
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new work entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := app.CreateWork(cmd.Context(), app.CreateWorkParams{
				Title:       args[0],
				Description: description,
				PadPath:     pad,
				Tags:        tags,
				NoSelect:    noSelect,
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
	cmd.Flags().StringVarP(&pad, "pad", "p", "",
		"Path to the pad — the per-entry folder for experimental/scratch work")
	cmd.Flags().StringSliceVarP(&tags, "tag", "t", nil,
		"Tag to attach (repeatable, comma-separated)")
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
				// Compact one-line format: <id>  <status>  <title>  [tags]
				// IDs are fixed-width (5 chars); statuses vary so we
				// pad to the longest enum value ("in-progress" = 11).
				// Tags are appended in brackets only when present so the
				// common (untagged) case stays clean.
				line := fmt.Sprintf("%s  %-11s  %s", e.ID, e.Status, e.Title)
				if len(e.Tags) > 0 {
					line += "  [" + strings.Join(e.Tags, ", ") + "]"
				}
				fmt.Fprintln(out, line)
			}
			return nil
		},
	}
}

//endregion

// region work show
func newWorkShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [id]",
		Short: "Show details of a work entry (defaults to the selected entry)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			entry, err := app.ShowWork(cmd.Context(), id)
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
	fmt.Fprintf(w, "Pad:          %s\n", orNone(e.PadPath))
	fmt.Fprintf(w, "Tags:         %s\n", orNone(strings.Join(e.Tags, ", ")))
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

func newWorkSelectedCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "selected",
		Short: "Show the currently selected work entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entry, err := app.GetSelectedWork(cmd.Context())
			if err != nil {
				return err
			}
			if all {
				printWorkEntry(cmd.OutOrStdout(), entry)
				return nil
			}
			printWorkEntryCompact(cmd.OutOrStdout(), entry)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false,
		"Show full details (same view as `work show`)")
	return cmd
}

// printWorkEntryCompact writes a single-line summary of e to w in
// the form:
//
//	<id>: <title> [tag1, tag2] (created YYYY-MM-DD, status: <status>)
//
// The bracketed tag list is omitted when the entry has no tags so
// the line stays tight in the common case.
func printWorkEntryCompact(w io.Writer, e core.WorkEntry) {
	const dateFmt = "2006-01-02"
	tagsPart := ""
	if len(e.Tags) > 0 {
		tagsPart = " [" + strings.Join(e.Tags, ", ") + "]"
	}
	fmt.Fprintf(w, "%s: %s%s (created %s, status: %s)\n",
		e.ID, e.Title, tagsPart,
		e.CreatedAt.UTC().Format(dateFmt), e.Status)
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

func newWorkTagCmd() *cobra.Command {
	var remove bool
	cmd := &cobra.Command{
		Use:   "tag [id] <tag>",
		Short: "Add (or with --remove, drop) a tag on a work entry; defaults to the selected entry when only <tag> is given",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Two-arg form: <id> <tag>. One-arg form: just <tag>,
			// in which case the app layer resolves the selected entry.
			id, rawTag := "", args[0]
			if len(args) == 2 {
				id, rawTag = args[0], args[1]
			}

			if remove {
				resolvedID, name, err := app.UntagWork(cmd.Context(), id, rawTag)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Removed tag %q from %s\n", name, resolvedID)
				return nil
			}
			resolvedID, name, err := app.TagWork(cmd.Context(), id, rawTag)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added tag %q to %s\n", name, resolvedID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&remove, "remove", false,
		"Remove the tag instead of adding it")
	return cmd
}
