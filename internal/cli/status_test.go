package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lydietoure/orbit/internal/app"
	"github.com/lydietoure/orbit/internal/core"
)

// TestPrintStatusOverview_Empty checks the empty-state rendering: no
// selection and no active entries produce the friendly placeholders.
func TestPrintStatusOverview_Empty(t *testing.T) {
	var buf bytes.Buffer
	printStatusOverview(&buf, app.StatusOverview{})

	out := buf.String()
	if !strings.Contains(out, "Selected: None selected") {
		t.Errorf("missing 'None selected' line:\n%s", out)
	}
	if !strings.Contains(out, "No active work entries.") {
		t.Errorf("missing empty active message:\n%s", out)
	}
}

// TestPrintStatusOverview_Populated checks that the selected entry and
// active entries (with tags) are rendered, and that truncation is
// surfaced when ActiveTotal exceeds the displayed slice.
func TestPrintStatusOverview_Populated(t *testing.T) {
	selected := core.WorkEntry{ID: "abc12", Title: "Add caching", Status: core.StatusInProgress}
	overview := app.StatusOverview{
		Selected: &selected,
		Active: []core.WorkEntry{
			{ID: "abc12", Title: "Add caching", Status: core.StatusInProgress, Tags: []string{"project:payments"}},
			{ID: "def34", Title: "Write design doc", Status: core.StatusNew},
		},
		ActiveTotal: 5,
	}

	var buf bytes.Buffer
	printStatusOverview(&buf, overview)
	out := buf.String()

	for _, want := range []string{
		`Selected: abc12 "Add caching" (in-progress)`,
		"Active work entries (5):",
		"abc12  in-progress  Add caching  [project:payments]",
		"def34  new          Write design doc",
		"... and 3 more",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
