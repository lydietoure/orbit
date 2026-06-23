package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/lydietoure/orbit/internal/app"
	"github.com/lydietoure/orbit/internal/core"
	"github.com/spf13/cobra"
)

// getCmdStatus builds the top-level `orbit status` command — a quick,
// at-a-glance dashboard of the current state. Like the other RunEs in
// this package it is intentionally thin: call ONE app function and
// format the result; all orchestration lives in the app layer.
func getCmdStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show a quick overview of current state",
		Long: "Print an at-a-glance dashboard: the currently selected work " +
			"entry and the active entries (status new or in-progress).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			overview, err := app.Status(cmd.Context())
			if err != nil {
				return err
			}
			printStatusOverview(cmd.OutOrStdout(), overview)
			return nil
		},
	}
}

// printStatusOverview renders the status dashboard to w. The selected
// entry is shown first and prominently; the active entries follow in
// the same compact one-line format used by `orbit work list`.
func printStatusOverview(w io.Writer, o app.StatusOverview) {
	if o.Selected != nil {
		s := o.Selected
		fmt.Fprintf(w, "Selected: %s %q (%s)\n", s.ID, s.Title, s.Status)
	} else {
		fmt.Fprintln(w, "Selected: None selected")
	}

	fmt.Fprintln(w)

	if len(o.Active) == 0 {
		fmt.Fprintln(w, "No active work entries.")
		return
	}

	fmt.Fprintf(w, "Active work entries (%d):\n", o.ActiveTotal)
	for _, e := range o.Active {
		fmt.Fprintln(w, statusActiveLine(e))
	}
	if o.ActiveTotal > len(o.Active) {
		fmt.Fprintf(w, "... and %d more\n", o.ActiveTotal-len(o.Active))
	}
}

// statusActiveLine formats one active entry as a compact line:
//
//	<id>  <status>  <title>  [tags]
//
// Mirrors `orbit work list` so the two views stay visually consistent.
// Tags are appended in brackets only when present.
func statusActiveLine(e core.WorkEntry) string {
	line := fmt.Sprintf("%s  %-11s  %s", e.ID, e.Status, e.Title)
	if len(e.Tags) > 0 {
		line += "  [" + strings.Join(e.Tags, ", ") + "]"
	}
	return line
}
