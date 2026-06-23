package cli

import (
	"fmt"

	"github.com/lydietoure/orbit/internal/app"
	"github.com/spf13/cobra"
)

// getCmdTags builds the top-level `orbit tags` command: a global
// overview of the free-form tag vocabulary with per-tag work-entry
// counts, alphabetical. Reserved `project:*` / `owner:*` tags are
// excluded by the app layer so they aren't surfaced twice — they have
// their own `work project` / `work owner` views.
//
// Like the other read commands, the RunE is tiny: call one app
// function and format the result.
func getCmdTags() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List all tags with how many work entries use each",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			counts, err := app.ListAllTags(cmd.Context())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(counts) == 0 {
				fmt.Fprintln(out, "No tags yet. Add one with `orbit work tag <tag>`.")
				return nil
			}
			for _, tc := range counts {
				fmt.Fprintf(out, "%s  (%d)\n", tc.Name, tc.Count)
			}
			return nil
		},
	}
}
