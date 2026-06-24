package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/lydietoure/orbit/internal/core"
)

// TestWriteArtifactLines checks the shared list renderer used by both
// `orbit link` and `orbit work show`.
func TestWriteArtifactLines(t *testing.T) {
	var buf bytes.Buffer
	writeArtifactLines(&buf, []core.Artifact{
		{Type: core.ArtifactBranch, Value: "feature/x"},
		{Type: core.ArtifactWorkItem, Value: "https://ado/wi/1"},
		{Type: core.ArtifactNote, Value: "/n/today.md"},
	})

	out := buf.String()
	for _, want := range []string{
		"  branch    feature/x",
		"  workitem  https://ado/wi/1",
		"  note      /n/today.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestPrintWorkEntry_SurfacesLinks verifies `work show` renders linked
// artifacts (including note artifacts) and the "(none)" placeholder
// when empty.
func TestPrintWorkEntry_SurfacesLinks(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	entry := core.WorkEntry{
		ID:        "abc12",
		Title:     "Add caching",
		Status:    core.StatusInProgress,
		CreatedAt: now,
		UpdatedAt: now,
		Artifacts: []core.Artifact{
			{Type: core.ArtifactBranch, Value: "feature/x"},
			{Type: core.ArtifactNote, Value: "/n/today.md"},
		},
	}

	var buf bytes.Buffer
	printWorkEntry(&buf, entry)
	out := buf.String()

	for _, want := range []string{
		"Artifacts:",
		"  branch    feature/x",
		"  note      /n/today.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}

	// Empty entry shows the (none) placeholder.
	buf.Reset()
	printWorkEntry(&buf, core.WorkEntry{ID: "def34", Title: "empty", Status: core.StatusNew, CreatedAt: now, UpdatedAt: now})
	out = buf.String()
	if want := "Artifacts:    (none)"; !strings.Contains(out, want) {
		t.Errorf("empty show output missing %q:\n%s", want, out)
	}
}
