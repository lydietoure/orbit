package cli

import (
	"fmt"
	"io"

	"github.com/lydietoure/orbit/internal/app"
	"github.com/lydietoure/orbit/internal/core"
	"github.com/spf13/cobra"
)

// getCmdLink builds the top-level `orbit link` command. Linking is a
// frequent action, so it lives at the root (not under `orbit work`).
//
// One invocation does exactly one thing, selected by which flag is set:
//
//	orbit link [id] --branch <name>      add a branch artifact
//	orbit link [id] --pr <url>           add a PR artifact   (and so on)
//	orbit link [id] --note <path>        add a note artifact
//	orbit link [id] --<type> <v> --remove   detach instead of attach
//	orbit link [id]                      list everything linked
//
// With no type flag the command lists; `--remove` is only meaningful
// alongside a type flag. The id is optional and defaults to the
// selected entry, matching `orbit work show` / `work tag`. The RunE
// stays thin: it works out the single intent, then calls one app
// function.
func getCmdLink() *cobra.Command {
	// One string var per artifact type, plus the remove toggle.
	// Binding each flag to its own var keeps the surface declarative;
	// cmd.Flags().Changed(name) tells us which was set.
	var (
		branch, pr, workitem string
		repo, dir, file      string
		urlVal, note, custom string
		remove               bool
	)

	cmd := &cobra.Command{
		Use:   "link [id]",
		Short: "Link artifacts to a work entry (defaults to the selected entry)",
		Long: "Attach external references to a work entry, turning it into a hub " +
			"that points at everything you touched.\n\n" +
			"Artifacts are typed references (branch, pr, workitem, repo, dir, " +
			"file, url, note, custom); a note points at a markdown file you " +
			"maintain elsewhere. Local paths are stored absolute and only " +
			"referenced — a path that doesn't exist yet is a warning, not an error.\n\n" +
			"Pick one thing per invocation with a type flag; add `--remove` to " +
			"detach it. With no type flag, the linked references are listed. The " +
			"id is optional and falls back to the selected entry.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := optionalID(args)

			// Map each type flag to its artifact type and bound value.
			specs := []struct {
				name string
				typ  core.ArtifactType
				val  *string
			}{
				{"branch", core.ArtifactBranch, &branch},
				{"pr", core.ArtifactPR, &pr},
				{"workitem", core.ArtifactWorkItem, &workitem},
				{"repo", core.ArtifactRepo, &repo},
				{"dir", core.ArtifactDir, &dir},
				{"file", core.ArtifactFile, &file},
				{"url", core.ArtifactURL, &urlVal},
				{"note", core.ArtifactNote, &note},
				{"custom", core.ArtifactCustom, &custom},
			}

			// Collect the set type flags so we can reject ambiguous
			// invocations (more than one thing at a time).
			var chosen []struct {
				typ core.ArtifactType
				val string
			}
			for _, s := range specs {
				if cmd.Flags().Changed(s.name) {
					chosen = append(chosen, struct {
						typ core.ArtifactType
						val string
					}{s.typ, *s.val})
				}
			}

			selectors := len(chosen)
			if selectors > 1 {
				return &UsageError{fmt.Errorf("link one thing at a time: pass a single type flag")}
			}
			if selectors == 0 {
				if remove {
					return &UsageError{fmt.Errorf("specify what to remove with a type flag (e.g. --branch <name>)")}
				}
				return runLinkList(cmd, id)
			}

			return runLinkArtifact(cmd, id, chosen[0].typ, chosen[0].val, remove)
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "", "Link a git branch name")
	cmd.Flags().StringVar(&pr, "pr", "", "Link a pull-request URL")
	cmd.Flags().StringVar(&workitem, "workitem", "", "Link an issue or work-item URL")
	cmd.Flags().StringVar(&repo, "repo", "", "Link a local git repository path")
	cmd.Flags().StringVar(&dir, "dir", "", "Link a local directory path (non-repo)")
	cmd.Flags().StringVar(&file, "file", "", "Link a local file path")
	cmd.Flags().StringVar(&urlVal, "url", "", "Link any other URL")
	cmd.Flags().StringVar(&note, "note", "", "Link a markdown note file")
	cmd.Flags().StringVar(&custom, "custom", "", "Link a freeform custom reference")
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove the reference instead of adding it")
	return cmd
}

// runLinkArtifact adds or removes a single typed artifact and echoes
// the mutation. A missing-path warning (add only) goes to stderr so
// stdout stays clean for scripting.
func runLinkArtifact(cmd *cobra.Command, id string, t core.ArtifactType, value string, remove bool) error {
	if remove {
		resolvedID, stored, err := app.UnlinkArtifact(cmd.Context(), id, t, value)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Unlinked %s %q from %s\n", t, stored, resolvedID)
		return nil
	}
	resolvedID, stored, warning, err := app.LinkArtifact(cmd.Context(), id, t, value)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Linked %s %q to %s\n", t, stored, resolvedID)
	if warning != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", warning)
	}
	return nil
}

// runLinkList prints everything linked to an entry, or a friendly
// note when there's nothing yet.
func runLinkList(cmd *cobra.Command, id string) error {
	resolvedID, artifacts, err := app.ListLinks(cmd.Context(), id)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(artifacts) == 0 {
		fmt.Fprintf(out, "%s has no links yet.\n", resolvedID)
		return nil
	}
	fmt.Fprintln(out, "Artifacts:")
	writeArtifactLines(out, artifacts)
	return nil
}

// artifactTypeWidth is the column width for the artifact type in list
// output — the longest type value ("workitem") is 8 characters.
const artifactTypeWidth = 8

// writeArtifactLines renders one indented "<type>  <value>" line per
// artifact.
func writeArtifactLines(w io.Writer, artifacts []core.Artifact) {
	for _, a := range artifacts {
		fmt.Fprintf(w, "  %-*s  %s\n", artifactTypeWidth, a.Type, a.Value)
	}
}
