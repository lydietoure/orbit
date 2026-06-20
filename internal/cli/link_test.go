package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/lydietoure/orbit/internal/core"
)

// TestWriteArtifactAndNoteLines checks the shared list renderers used
// by both `orbit link` and `orbit work show`.
func TestWriteArtifactAndNoteLines(t *testing.T) {
	var buf bytes.Buffer
	writeArtifactLines(&buf, []core.Artifact{
		{Type: core.ArtifactBranch, Value: "feature/x"},
		{Type: core.ArtifactWorkItem, Value: "https://ado/wi/1"},
	})
	writeNoteLines(&buf, []core.Note{
		{Date: "2026-06-20", Path: "/n/today.md"},
	})

	out := buf.String()
	for _, want := range []string{
		"  branch    feature/x",
		"  workitem  https://ado/wi/1",
		"  2026-06-20  /n/today.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestPrintWorkEntry_SurfacesLinks verifies `work show` renders linked
// artifacts and notes (and the "(none)" placeholders when empty).
func TestPrintWorkEntry_SurfacesLinks(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	entry := core.WorkEntry{
		ID:        "abc12",
		Title:     "Add caching",
		Status:    core.StatusInProgress,
		CreatedAt: now,
		UpdatedAt: now,
		Artifacts: []core.Artifact{{Type: core.ArtifactBranch, Value: "feature/x"}},
		Notes:     []core.Note{{Date: "2026-06-20", Path: "/n/today.md"}},
	}

	var buf bytes.Buffer
	printWorkEntry(&buf, entry)
	out := buf.String()

	for _, want := range []string{
		"Artifacts:",
		"  branch    feature/x",
		"Notes:",
		"  2026-06-20  /n/today.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}

	// Empty entry shows the (none) placeholders.
	buf.Reset()
	printWorkEntry(&buf, core.WorkEntry{ID: "def34", Title: "empty", Status: core.StatusNew, CreatedAt: now, UpdatedAt: now})
	out = buf.String()
	for _, want := range []string{"Artifacts:    (none)", "Notes:        (none)"} {
		if !strings.Contains(out, want) {
			t.Errorf("empty show output missing %q:\n%s", want, out)
		}
	}
}
