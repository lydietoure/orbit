package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
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
		Use:     "work",
		Aliases: []string{"w"},
		Short:   "Manage work entries",
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
		newWorkProjectCmd(),
		newWorkOwnerCmd(),
		newWorkPadCmd(),
	)
	return cmd
}

// region work new
func newWorkNewCmd() *cobra.Command {
	var (
		description string
		pad         string
		noDock      bool
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
				NoDock:      noDock,
				Tags:        tags,
				NoSelect:    noSelect,
			})
			padExisted := errors.Is(err, app.ErrPadAlreadyExisted)
			if padExisted {
				err = nil // success-with-warning sentinel
			}
			if err != nil {
				return err
			}
			verb := "Created and selected"
			if noSelect {
				verb = "Created"
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s %s: %q\n", verb, entry.ID, entry.Title)
			if padExisted {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"Note: pad directory %s already existed and is being reused as-is.\n",
					entry.PadPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "",
		"Longer explanation of this work")
	cmd.Flags().StringVarP(&pad, "pad", "p", "",
		"Path to the pad — the per-entry folder for experimental/scratch work")
	cmd.Flags().BoolVar(&noDock, "no-dock", false,
		"Ignore the dock root and create the pad relative to the current directory (only meaningful with -p)")
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
				// Compact one-line format: <id>  <status>  <title>  <reserved>  [tags]
				// IDs are fixed-width (5 chars); statuses vary so we
				// pad to the longest enum value ("in-progress" = 11).
				// Reserved owner/project tags are surfaced separately
				// (see reservedSummary) so they don't get lost in the
				// plain-tag bracket, which is appended only when there
				// are plain tags so the common case stays clean.
				projects, owner, plain := core.PartitionReservedTags(e.Tags)
				line := fmt.Sprintf("%s  %-11s  %s", e.ID, e.Status, e.Title)
				if s := reservedSummary(owner, projects); s != "" {
					line += "  " + s
				}
				if len(plain) > 0 {
					line += "  [" + strings.Join(plain, ", ") + "]"
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
	projects, owner, plain := core.PartitionReservedTags(e.Tags)
	fmt.Fprintf(w, "ID:           %s\n", e.ID)
	fmt.Fprintf(w, "Title:        %s\n", e.Title)
	fmt.Fprintf(w, "Status:       %s\n", e.Status)
	if e.StatusReason != "" {
		fmt.Fprintf(w, "Reason:       %s\n", e.StatusReason)
	}
	fmt.Fprintf(w, "Description:  %s\n", orNone(e.Description))
	fmt.Fprintf(w, "Pad:          %s\n", orNone(e.PadPath))
	fmt.Fprintf(w, "Owner:        %s\n", orNone(owner))
	fmt.Fprintf(w, "Projects:     %s\n", orNone(strings.Join(projects, ", ")))
	fmt.Fprintf(w, "Tags:         %s\n", orNone(strings.Join(plain, ", ")))
	if len(e.Artifacts) == 0 {
		fmt.Fprintf(w, "Artifacts:    %s\n", orNone(""))
	} else {
		fmt.Fprintln(w, "Artifacts:")
		writeArtifactLines(w, e.Artifacts)
	}
	if len(e.Notes) == 0 {
		fmt.Fprintf(w, "Notes:        %s\n", orNone(""))
	} else {
		fmt.Fprintln(w, "Notes:")
		writeNoteLines(w, e.Notes)
	}
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

// region work delete
func newWorkDeleteCmd() *cobra.Command {
	var (
		yes   bool
		purge bool
	)
	cmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a work entry (defaults to the selected entry)",
		Long: "Delete a work entry from the database.\n\n" +
			"If [id] is omitted, the currently selected entry is deleted.\n\n" +
			"By default the pad folder on disk is left in place; the path is " +
			"reported so you can decide what to do with it. Pass --purge to " +
			"also remove the pad folder (irreversible).\n\n" +
			"Prompts for confirmation by default; pass --yes to skip the prompt " +
			"(scripts, piped input).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = args[0]
			}

			// Pre-load so the prompt names the entry the user is
			// about to drop (and so a typo'd id — or an unset
			// selection — fails before the prompt rather than
			// after a confused "yes"). The follow-up DeleteWork
			// call re-loads inside the transaction-less window;
			// that's fine — see the note on app.DeleteWork.
			preview, err := app.ShowWork(cmd.Context(), id)
			if err != nil {
				return err
			}

			// Combine the DB delete and the optional pad delete
			// into one prompt: the user is making one decision
			// ("trash this whole thing"), not two.
			willPurge := purge && preview.PadPath != ""
			if !yes {
				question := fmt.Sprintf("Delete work entry %s %q?", preview.ID, preview.Title)
				if willPurge {
					question = fmt.Sprintf(
						"Delete work entry %s %q AND remove pad folder %s?",
						preview.ID, preview.Title, preview.PadPath,
					)
				}
				ok, err := confirm(cmd.InOrStdin(), cmd.ErrOrStderr(), question)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			deleted, err := app.DeleteWork(cmd.Context(), preview.ID)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Deleted %s: %q\n", deleted.ID, deleted.Title)

			// Pad disposition. The DB row is gone at this point;
			// the pad folder is on disk only and its removal is a
			// best-effort step — but if the user explicitly asked
			// for --purge and we couldn't deliver, surface that
			// as an error so the exit code reflects reality and
			// the path is shown for manual cleanup.
			if deleted.PadPath == "" {
				return nil
			}
			if !purge {
				fmt.Fprintf(out,
					"Pad folder at %s left in place (use --purge to also remove it).\n",
					deleted.PadPath)
				return nil
			}
			if err := os.RemoveAll(deleted.PadPath); err != nil {
				return fmt.Errorf(
					"work entry deleted, but failed to remove pad folder %s: %w",
					deleted.PadPath, err,
				)
			}
			fmt.Fprintf(out, "Removed pad folder at %s\n", deleted.PadPath)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"Skip the confirmation prompt (intended for scripts)")
	cmd.Flags().BoolVar(&purge, "purge", false,
		"Also remove the pad folder from disk (irreversible)")
	return cmd
}

// endregion

// region work select
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
	projects, owner, plain := core.PartitionReservedTags(e.Tags)
	extras := reservedSummary(owner, projects)
	if len(plain) > 0 {
		if extras != "" {
			extras += " "
		}
		extras += "[" + strings.Join(plain, ", ") + "]"
	}
	if extras != "" {
		extras = " " + extras
	}
	fmt.Fprintf(w, "%s: %s%s (created %s, status: %s)\n",
		e.ID, e.Title, extras,
		e.CreatedAt.UTC().Format(dateFmt), e.Status)
}

// reservedSummary renders the reserved owner/project tags of an entry
// as a compact, self-describing string for the one-line views, e.g.
// "owner:work project:payments project:orbit". Returns "" when the
// entry has neither an owner nor any projects. Keeping the `owner:` /
// `project:` prefixes makes the segments unambiguous without inventing
// new sigils, while grouping them out of the plain-tag bracket keeps
// the two kinds visually distinct.
func reservedSummary(owner string, projects []string) string {
	parts := make([]string, 0, 1+len(projects))
	if owner != "" {
		parts = append(parts, core.OwnerTagPrefix+owner)
	}
	for _, p := range projects {
		parts = append(parts, core.ProjectTagPrefix+p)
	}
	return strings.Join(parts, " ")
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

//endregion

// region work tag
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

// TODO: add `work tag list`

//endregion

// region work project

// newWorkProjectCmd builds the `orbit work project` group: add, remove,
// and list the `project:*` tags on an entry. Projects are multi-valued
// (docs/DATA_MODEL.md), so each leaf does exactly one thing. The parent
// has no Run of its own — the strict-mode helper in strict.go makes a
// bare `orbit work project` exit 2 with the usual hint.
func newWorkProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Add, remove, or list the projects on a work entry",
	}
	cmd.AddCommand(
		newWorkProjectAddCmd(),
		newWorkProjectRemoveCmd(),
		newWorkProjectListCmd(),
	)
	return cmd
}

func newWorkProjectAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [id] <name>",
		Short: "Add a project to a work entry; defaults to the selected entry when only <name> is given",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, name := idAndValue(args)
			resolvedID, project, err := app.AddProject(cmd.Context(), id, name)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added project %q to %s\n", project, resolvedID)
			return nil
		},
	}
}

func newWorkProjectRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [id] <name>",
		Short: "Remove a project from a work entry; defaults to the selected entry when only <name> is given",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, name := idAndValue(args)
			resolvedID, project, err := app.RemoveProject(cmd.Context(), id, name)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed project %q from %s\n", project, resolvedID)
			return nil
		},
	}
}

func newWorkProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [id]",
		Short: "List the projects on a work entry (defaults to the selected entry)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := optionalID(args)
			resolvedID, projects, err := app.ListProjects(cmd.Context(), id)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(projects) == 0 {
				fmt.Fprintf(out, "%s has no projects.\n", resolvedID)
				return nil
			}
			for _, p := range projects {
				fmt.Fprintln(out, p)
			}
			return nil
		},
	}
}

//endregion

// region work owner

// newWorkOwnerCmd builds the `orbit work owner` group: add (set),
// remove (clear), and list (show) the single `owner:*` tag on an entry.
// Owner is single-valued (docs/DATA_MODEL.md); `add` replaces any
// existing owner atomically in the app layer. The parent has no Run of
// its own — strict.go makes a bare `orbit work owner` exit 2.
func newWorkOwnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "owner",
		Short: "Set, remove, or show the owner of a work entry",
	}
	cmd.AddCommand(
		newWorkOwnerAddCmd(),
		newWorkOwnerRemoveCmd(),
		newWorkOwnerListCmd(),
	)
	return cmd
}

func newWorkOwnerAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [id] <name>",
		Short: "Set the owner of a work entry, replacing any existing one; defaults to the selected entry when only <name> is given",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, name := idAndValue(args)
			resolvedID, owner, err := app.SetOwner(cmd.Context(), id, name)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set owner of %s to %q\n", resolvedID, owner)
			return nil
		},
	}
}

func newWorkOwnerRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [id]",
		Short: "Remove the owner tag from a work entry (defaults to the selected entry)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := optionalID(args)
			resolvedID, prev, err := app.ClearOwner(cmd.Context(), id)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if prev == "" {
				fmt.Fprintf(out, "%s has no owner to remove.\n", resolvedID)
				return nil
			}
			fmt.Fprintf(out, "Removed owner %q from %s\n", prev, resolvedID)
			return nil
		},
	}
}

func newWorkOwnerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [id]",
		Short: "Show the owner of a work entry (defaults to the selected entry)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := optionalID(args)
			resolvedID, owner, err := app.GetOwner(cmd.Context(), id)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if owner == "" {
				fmt.Fprintf(out, "%s has no owner.\n", resolvedID)
				return nil
			}
			fmt.Fprintln(out, owner)
			return nil
		},
	}
}

// idAndValue interprets the `[id] <value>` positional shape shared by
// the reserved-tag add/remove leaves. With two args the first is the
// entry id and the second the value; with one arg it's the value and
// the entry defaults to the selection (id "").
func idAndValue(args []string) (id, value string) {
	if len(args) == 2 {
		return args[0], args[1]
	}
	return "", args[0]
}

// optionalID interprets the `[id]` positional shape shared by the
// reserved-tag list leaves: the id when given, otherwise "" so the
// app layer falls back to the selected entry.
func optionalID(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return ""
}

//endregion

// region work pad

// newWorkPadCmd builds the `orbit work pad` group: focused commands
// for inspecting and changing the pad on a single entry. The parent
// has no Run of its own — the strict-mode helper in strict.go makes
// bare `orbit work pad` exit 2 with the usual hint.
func newWorkPadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pad",
		Short: "Inspect or change the pad folder attached to a work entry",
	}
	cmd.AddCommand(
		newWorkPadGetCmd(),
		newWorkPadSetCmd(),
		newWorkPadClearCmd(),
		newWorkPadShowCmd(),
	)
	return cmd
}

func newWorkPadGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [id]",
		Short: "Print only the pad path (defaults to the selected entry); exits non-zero when no pad is set",
		Long: "Print the absolute pad path on stdout with no decoration, so it can " +
			"be captured in scripts: `pad=$(orbit work pad get)`.\n\n" +
			"Exits with a non-zero status and a brief stderr message when " +
			"the entry has no pad set, so callers can distinguish unset from empty.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			entry, err := app.ShowWork(cmd.Context(), id)
			if err != nil {
				return err
			}
			if entry.PadPath == "" {
				return fmt.Errorf("entry %s has no pad set", entry.ID)
			}
			fmt.Fprintln(cmd.OutOrStdout(), entry.PadPath)
			return nil
		},
	}
}

func newWorkPadSetCmd() *cobra.Command {
	var noDock bool
	cmd := &cobra.Command{
		Use:   "set [id] <path>",
		Short: "Set (or change) the pad on a work entry; defaults to the selected entry when only <path> is given",
		Long: "Set the pad folder for a work entry. <path> follows the same " +
			"resolution rules as `orbit work new -p`: a bare name resolves " +
			"under the dock root, an absolute or `./`-prefixed path is used " +
			"as-is. The directory is provisioned if it doesn't exist; if it " +
			"already exists, it's adopted as-is with a note.\n\n" +
			"To remove the pad pointer (without deleting the folder on disk) " +
			"use `orbit work pad clear`.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Two-arg form: <id> <path>. One-arg form: just <path>,
			// in which case the app layer resolves the selected entry.
			id, rawPath := "", args[0]
			if len(args) == 2 {
				id, rawPath = args[0], args[1]
			}

			entry, err := app.SetPad(cmd.Context(), id, rawPath, noDock)
			padExisted := errors.Is(err, app.ErrPadAlreadyExisted)
			if padExisted {
				err = nil // success-with-warning sentinel
			}
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Pad for %s set to %s\n", entry.ID, entry.PadPath)
			if padExisted {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"Note: directory %s already existed and is being reused as-is.\n",
					entry.PadPath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noDock, "no-dock", false,
		"Ignore the dock root and resolve <path> relative to the current directory")
	return cmd
}

func newWorkPadClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear [id]",
		Short: "Clear the pad pointer on a work entry (defaults to the selected entry); does NOT touch the folder on disk",
		Long: "Remove the pad reference from a work entry. The directory on " +
			"disk is intentionally left alone — disk removal belongs to " +
			"`orbit work delete --purge`. Re-attach a pad later with " +
			"`orbit work pad set`.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			entry, err := app.SetPad(cmd.Context(), id, "", false)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pad cleared on %s\n", entry.ID)
			return nil
		},
	}
}

func newWorkPadShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [id]",
		Short: "Show the pad path and whether the directory exists on disk (defaults to the selected entry)",
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

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Entry:  %s %q\n", entry.ID, entry.Title)
			if entry.PadPath == "" {
				fmt.Fprintln(out, "Pad:    (none)")
				return nil
			}

			// Resolve "exists" with a quick stat — string answer
			// stays useful even on read errors (a permission issue
			// is reported alongside the path rather than as a
			// command failure, since the user is asking for info).
			status := "exists"
			if info, statErr := os.Stat(entry.PadPath); statErr != nil {
				if os.IsNotExist(statErr) {
					status = "missing"
				} else {
					status = fmt.Sprintf("stat error: %v", statErr)
				}
			} else if !info.IsDir() {
				status = "not a directory"
			}
			fmt.Fprintf(out, "Pad:    %s  (%s)\n", entry.PadPath, status)
			return nil
		},
	}
}

//endregion
