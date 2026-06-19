package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestConfirm_AcceptsYesVariants locks in the "yes" surface: y,
// yes, Y, YES, surrounding whitespace. Anything outside this set
// must NOT count as yes (covered in the rejection test below).
func TestConfirm_AcceptsYesVariants(t *testing.T) {
	cases := []string{"y\n", "Y\n", "yes\n", "YES\n", "  y \n", "  yes  \n"}
	for _, in := range cases {
		t.Run(strings.TrimSpace(in), func(t *testing.T) {
			out := &bytes.Buffer{}
			got, err := confirm(strings.NewReader(in), out, "ok?")
			if err != nil {
				t.Fatalf("confirm: %v", err)
			}
			if !got {
				t.Errorf("input %q returned false, want true", in)
			}
		})
	}
}

// TestConfirm_RejectsEverythingElse covers the safe-default
// contract: a default-No prompt must reject blank input, "n", "no",
// other words, and even truthy-looking aliases like "1" or "true".
// The whole point of [y/N] is that only an explicit y/yes flips it.
func TestConfirm_RejectsEverythingElse(t *testing.T) {
	cases := []string{
		"\n",            // bare Enter
		"n\n",           // explicit no
		"N\n",           // explicit no, capitalized
		"no\n",          // explicit no, long form
		"NO\n",          // explicit no, long form, capitalized
		"yep\n",         // close-but-no synonym
		"yeah\n",        // close-but-no synonym
		"true\n",        // truthy literal we explicitly don't honor
		"1\n",           // truthy literal we explicitly don't honor
		"banana\n",      // unrelated word
		"y something\n", // y followed by other text — not just "y"
	}
	for _, in := range cases {
		t.Run(strings.TrimSpace(in), func(t *testing.T) {
			out := &bytes.Buffer{}
			got, err := confirm(strings.NewReader(in), out, "ok?")
			if err != nil {
				t.Fatalf("confirm: %v", err)
			}
			if got {
				t.Errorf("input %q returned true, want false", in)
			}
		})
	}
}

// TestConfirm_EOFIsNo: closing stdin without typing anything (e.g.
// `orbit work delete xyz </dev/null`) must NOT be treated as yes.
// EOF is the absence of an answer, and our default is No.
func TestConfirm_EOFIsNo(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := confirm(strings.NewReader(""), out, "ok?")
	if err != nil {
		t.Fatalf("confirm on EOF returned err: %v", err)
	}
	if got {
		t.Error("EOF returned true, want false")
	}
}

// TestConfirm_WritesPrompt: the prompt text plus " [y/N]: " must
// land on the output writer so the user can see what they're being
// asked. Other tests don't care about the prompt body; this one is
// the dedicated check.
func TestConfirm_WritesPrompt(t *testing.T) {
	out := &bytes.Buffer{}
	if _, err := confirm(strings.NewReader("n\n"), out, "delete xyz?"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	want := "delete xyz? [y/N]: "
	if got := out.String(); got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}
}
