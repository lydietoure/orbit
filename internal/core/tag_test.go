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
