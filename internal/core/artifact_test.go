package core

import (
	"strings"
	"testing"
	"time"
)

func TestParseArtifactType(t *testing.T) {
	cases := map[string]ArtifactType{
		"branch":     ArtifactBranch,
		"PR":         ArtifactPR,
		"  workitem": ArtifactWorkItem,
		"Repo":       ArtifactRepo,
		"dir":        ArtifactDir,
		"file":       ArtifactFile,
		"url":        ArtifactURL,
		"custom":     ArtifactCustom,
	}
	for raw, want := range cases {
		got, err := ParseArtifactType(raw)
		if err != nil {
			t.Errorf("ParseArtifactType(%q) error: %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("ParseArtifactType(%q) = %q, want %q", raw, got, want)
		}
	}

	if _, err := ParseArtifactType("nope"); err == nil {
		t.Error("ParseArtifactType(\"nope\") = nil error, want error")
	}
}

func TestArtifactTypeClassification(t *testing.T) {
	for _, t2 := range []ArtifactType{ArtifactRepo, ArtifactDir, ArtifactFile} {
		if !t2.IsLocalPath() {
			t.Errorf("%q.IsLocalPath() = false, want true", t2)
		}
		if t2.IsURL() {
			t.Errorf("%q.IsURL() = true, want false", t2)
		}
	}
	for _, t2 := range []ArtifactType{ArtifactPR, ArtifactWorkItem, ArtifactURL} {
		if !t2.IsURL() {
			t.Errorf("%q.IsURL() = false, want true", t2)
		}
		if t2.IsLocalPath() {
			t.Errorf("%q.IsLocalPath() = true, want false", t2)
		}
	}
}

func TestNormalizeValue_TrimsAndRejectsEmpty(t *testing.T) {
	got, err := ArtifactBranch.NormalizeValue("  feature/x  ")
	if err != nil {
		t.Fatalf("NormalizeValue: %v", err)
	}
	if got != "feature/x" {
		t.Errorf("NormalizeValue = %q, want %q", got, "feature/x")
	}

	if _, err := ArtifactCustom.NormalizeValue("   "); err == nil {
		t.Error("NormalizeValue(blank) = nil error, want error")
	}
}

func TestNormalizeValue_URLTypes(t *testing.T) {
	valid := []string{
		"https://github.com/o/r/pull/1",
		"http://example.com",
		"ssh://git@host/path",
	}
	for _, v := range valid {
		if _, err := ArtifactPR.NormalizeValue(v); err != nil {
			t.Errorf("NormalizeValue(%q) error: %v, want nil", v, err)
		}
	}

	invalid := []string{"not a url", "just-text", "/local/path", ""}
	for _, v := range invalid {
		if _, err := ArtifactURL.NormalizeValue(v); err == nil {
			t.Errorf("NormalizeValue(%q) = nil error, want error", v)
		}
	}
}

func TestNormalizeValue_LocalPathNotAbsolutized(t *testing.T) {
	// core stays pure: it must not touch the filesystem or rewrite a
	// relative path to absolute (that is the app layer's job).
	got, err := ArtifactFile.NormalizeValue("./relative/path")
	if err != nil {
		t.Fatalf("NormalizeValue: %v", err)
	}
	if got != "./relative/path" {
		t.Errorf("NormalizeValue = %q, want it left relative", got)
	}
}

func TestNormalizeNoteDate(t *testing.T) {
	got, err := NormalizeNoteDate("2026-06-20")
	if err != nil {
		t.Fatalf("NormalizeNoteDate: %v", err)
	}
	if got != "2026-06-20" {
		t.Errorf("NormalizeNoteDate = %q, want 2026-06-20", got)
	}

	// Empty defaults to today (UTC).
	got, err = NormalizeNoteDate("")
	if err != nil {
		t.Fatalf("NormalizeNoteDate(empty): %v", err)
	}
	want := time.Now().UTC().Format(NoteDateLayout)
	if got != want {
		t.Errorf("NormalizeNoteDate(empty) = %q, want today %q", got, want)
	}

	for _, bad := range []string{"2026/06/20", "June 20", "2026-13-01", "20-06-2026"} {
		if _, err := NormalizeNoteDate(bad); err == nil {
			t.Errorf("NormalizeNoteDate(%q) = nil error, want error", bad)
		} else if !strings.Contains(err.Error(), "YYYY-MM-DD") {
			t.Errorf("NormalizeNoteDate(%q) error %q should mention the wanted format", bad, err)
		}
	}
}
