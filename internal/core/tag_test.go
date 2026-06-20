package core

import "testing"

func TestNormalizeTagName_LowercasesAndTrims(t *testing.T) {
	cases := map[string]string{
		"caching":          "caching",
		"  Caching  ":      "caching",
		"PROJECT:Payments": "project:payments",
		"owner:Work":       "owner:work",
		"code review":      "code review", // internal whitespace ok
	}
	for input, want := range cases {
		got, err := NormalizeTagName(input)
		if err != nil {
			t.Errorf("NormalizeTagName(%q): unexpected error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeTagName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeTagName_RejectsEmpty(t *testing.T) {
	for _, input := range []string{"", "   ", "\t\n"} {
		if _, err := NormalizeTagName(input); err == nil {
			t.Errorf("NormalizeTagName(%q): expected error, got nil", input)
		}
	}
}

func TestNormalizeTagName_RejectsComma(t *testing.T) {
	if _, err := NormalizeTagName("a,b"); err == nil {
		t.Fatal("expected error for tag containing comma, got nil")
	}
}

func TestProjectTagName(t *testing.T) {
	cases := map[string]string{
		"payments":        "project:payments",
		"  Payments  ":    "project:payments",
		"project:orbit":   "project:orbit", // tolerate an explicit prefix
		"PROJECT:Caching": "project:caching",
	}
	for input, want := range cases {
		got, err := ProjectTagName(input)
		if err != nil {
			t.Errorf("ProjectTagName(%q): unexpected error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ProjectTagName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOwnerTagName(t *testing.T) {
	cases := map[string]string{
		"work":         "owner:work",
		"  Personal  ": "owner:personal",
		"owner:work":   "owner:work", // tolerate an explicit prefix
	}
	for input, want := range cases {
		got, err := OwnerTagName(input)
		if err != nil {
			t.Errorf("OwnerTagName(%q): unexpected error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("OwnerTagName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReservedTagName_RejectsEmptyAndPrefixOnly(t *testing.T) {
	for _, input := range []string{"", "   ", "project:"} {
		if _, err := ProjectTagName(input); err == nil {
			t.Errorf("ProjectTagName(%q): expected error, got nil", input)
		}
	}
	for _, input := range []string{"", "  ", "owner:"} {
		if _, err := OwnerTagName(input); err == nil {
			t.Errorf("OwnerTagName(%q): expected error, got nil", input)
		}
	}
}

func TestPartitionReservedTags(t *testing.T) {
	// Mirrors the storage layer's alphabetical ordering.
	tags := []string{"caching", "owner:work", "perf", "project:orbit", "project:payments"}
	projects, owner, plain := PartitionReservedTags(tags)

	wantProjects := []string{"orbit", "payments"}
	if len(projects) != len(wantProjects) {
		t.Fatalf("projects = %v, want %v", projects, wantProjects)
	}
	for i := range wantProjects {
		if projects[i] != wantProjects[i] {
			t.Errorf("projects[%d] = %q, want %q", i, projects[i], wantProjects[i])
		}
	}
	if owner != "work" {
		t.Errorf("owner = %q, want %q", owner, "work")
	}
	wantPlain := []string{"caching", "perf"}
	if len(plain) != len(wantPlain) {
		t.Fatalf("plain = %v, want %v", plain, wantPlain)
	}
	for i := range wantPlain {
		if plain[i] != wantPlain[i] {
			t.Errorf("plain[%d] = %q, want %q", i, plain[i], wantPlain[i])
		}
	}
}

func TestPartitionReservedTags_EmptyAndNoneReserved(t *testing.T) {
	projects, owner, plain := PartitionReservedTags(nil)
	if projects != nil || owner != "" || plain != nil {
		t.Errorf("nil input: got projects=%v owner=%q plain=%v", projects, owner, plain)
	}

	projects, owner, plain = PartitionReservedTags([]string{"caching", "perf"})
	if owner != "" || len(projects) != 0 {
		t.Errorf("no reserved: got projects=%v owner=%q", projects, owner)
	}
	if len(plain) != 2 {
		t.Errorf("plain = %v, want both plain tags", plain)
	}
}

// TestPartitionReservedTags_MultipleOwnersFirstWins documents the
// defensive behavior when more than one owner tag is present (the app
// layer prevents this, but the partition must still be deterministic).
func TestPartitionReservedTags_MultipleOwnersFirstWins(t *testing.T) {
	_, owner, _ := PartitionReservedTags([]string{"owner:personal", "owner:work"})
	if owner != "personal" {
		t.Errorf("owner = %q, want first in order %q", owner, "personal")
	}
}
