package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// buildTestTree mirrors a real cobra layout in miniature: a root
// with one group ("config") that has one leaf ("dock"). Used by the
// strict-marking tests so they don't poke at the package-level
// rootCmd singleton.
func buildTestTree() (*cobra.Command, *cobra.Command, *cobra.Command) {
	root := &cobra.Command{Use: "tool"}
	group := &cobra.Command{Use: "config"}
	leaf := &cobra.Command{
		Use:  "dock",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return nil },
	}
	group.AddCommand(leaf)
	root.AddCommand(group)
	return root, group, leaf
}

func TestMarkGroupAsStrict_InstallsArgsAndRunE_OnPureGroup(t *testing.T) {
	_, group, _ := buildTestTree()
	markGroupAsStrict(group)
	if group.Args == nil {
		t.Error("Args not installed on pure group")
	}
	if group.RunE == nil {
		t.Error("RunE not installed on pure group")
	}
}

func TestMarkGroupAsStrict_LeavesLeafAlone(t *testing.T) {
	_, _, leaf := buildTestTree()
	prevArgs := fmt.Sprintf("%p", leaf.Args)
	prevRunE := fmt.Sprintf("%p", leaf.RunE)
	markGroupAsStrict(leaf)
	if fmt.Sprintf("%p", leaf.Args) != prevArgs {
		t.Error("Args was overwritten on leaf command")
	}
	if fmt.Sprintf("%p", leaf.RunE) != prevRunE {
		t.Error("RunE was overwritten on leaf command")
	}
}

func TestMarkGroupAsStrict_LeavesGroupWithOwnRunAlone(t *testing.T) {
	group := &cobra.Command{Use: "g"}
	group.AddCommand(&cobra.Command{Use: "sub"})
	group.RunE = func(*cobra.Command, []string) error { return nil }
	prev := fmt.Sprintf("%p", group.RunE)
	markGroupAsStrict(group)
	if fmt.Sprintf("%p", group.RunE) != prev {
		t.Error("RunE was overwritten on group that already had one")
	}
	if group.Args != nil {
		t.Error("Args was installed despite existing RunE")
	}
}

func TestStrictGroup_RunE_ReturnsUsageError(t *testing.T) {
	_, group, _ := buildTestTree()
	markGroupAsStrict(group)
	err := group.RunE(group, nil)
	if AsUsageError(err) == nil {
		t.Errorf("RunE returned %v, want a *UsageError", err)
	}
	if !strings.Contains(err.Error(), "subcommand is required") {
		t.Errorf("error %q should mention `subcommand is required`", err)
	}
}

func TestStrictGroup_Args_RejectsUnknownSubcommand(t *testing.T) {
	_, group, _ := buildTestTree()
	markGroupAsStrict(group)
	err := group.Args(group, []string{"bogus"})
	if AsUsageError(err) == nil {
		t.Errorf("Args returned %v, want a *UsageError", err)
	}
	if !strings.Contains(err.Error(), `"bogus"`) {
		t.Errorf("error %q should mention the unknown arg", err)
	}
}

func TestStrictGroup_Args_AcceptsZeroArgs(t *testing.T) {
	// Zero args isn't an unknown-subcommand case; the RunE handles
	// the "no subcommand given" path. Args must let zero args
	// through so cobra reaches RunE.
	_, group, _ := buildTestTree()
	markGroupAsStrict(group)
	if err := group.Args(group, nil); err != nil {
		t.Errorf("Args rejected zero args: %v", err)
	}
}

func TestAsUsageError_DetectsWrappedSentinel(t *testing.T) {
	inner := &UsageError{err: errors.New("inner")}
	wrapped := fmt.Errorf("outer: %w", inner)
	got := AsUsageError(wrapped)
	if got == nil {
		t.Fatal("AsUsageError returned nil for wrapped *UsageError")
	}
	if got != inner {
		t.Errorf("AsUsageError returned %p, want %p", got, inner)
	}
}

func TestAsUsageError_ReturnsNilForPlainError(t *testing.T) {
	if AsUsageError(errors.New("not a usage error")) != nil {
		t.Error("AsUsageError returned non-nil for a plain error")
	}
}

func TestMarkAllGroupsAsStrict_AppliesToWholeTree(t *testing.T) {
	root, group, leaf := buildTestTree()
	markAllGroupsAsStrict(root)

	if root.Args == nil || root.RunE == nil {
		t.Error("root group not strict-marked")
	}
	if group.Args == nil || group.RunE == nil {
		t.Error("nested group not strict-marked")
	}
	// Leaf should be untouched.
	if leaf.RunE == nil {
		t.Error("leaf RunE was cleared (shouldn't happen)")
	}
	if err := leaf.RunE(leaf, nil); err != nil {
		t.Errorf("leaf RunE was replaced; got error: %v", err)
	}
}
