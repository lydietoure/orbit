package cli

// This file makes orbit's command groups strict: bare invocations
// (`orbit`, `orbit config`, `orbit config dock`) and unknown
// subcommands (`orbit config bogus`) both fail with a UsageError
// instead of cobra's default "print help, exit 0".
//
// Layering: the typed error lives here; main.go inspects it via
// AsUsageError to pick exit code 2 (POSIX convention for incorrect
// usage) instead of the 1 we use for runtime failures.

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// UsageError marks an error as caused by incorrect command-line
// usage (unknown subcommand, missing required subcommand, etc.)
// rather than a runtime failure.
type UsageError struct {
	err error
}

func (u *UsageError) Error() string { return u.err.Error() }
func (u *UsageError) Unwrap() error { return u.err }

// AsUsageError returns a *UsageError if err is one or wraps one,
// otherwise nil. Used by main() to pick the exit code.
func AsUsageError(err error) *UsageError {
	var u *UsageError
	if errors.As(err, &u) {
		return u
	}
	return nil
}

// markGroupAsStrict configures cmd so that bare invocations and
// unknown subcommands fail with a UsageError. Only command groups
// (commands with subcommands and no Run/RunE of their own) are
// marked; leaf commands keep their own argument validators.
func markGroupAsStrict(cmd *cobra.Command) {
	if cmd.Run != nil || cmd.RunE != nil {
		return
	}
	if !cmd.HasSubCommands() {
		return
	}
	path := cmd.CommandPath()
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil // RunE handles the "no subcommand given" case
		}
		return &UsageError{fmt.Errorf(
			"unknown command %q for %q (run `%s --help` for valid subcommands)",
			args[0], path, path,
		)}
	}
	cmd.RunE = func(_ *cobra.Command, _ []string) error {
		return &UsageError{fmt.Errorf(
			"a subcommand is required for %q (run `%s --help` for valid subcommands)",
			path, path,
		)}
	}
}

// markAllGroupsAsStrict walks cmd and every descendant applying
// markGroupAsStrict. Call once after all subcommands are registered.
func markAllGroupsAsStrict(cmd *cobra.Command) {
	markGroupAsStrict(cmd)
	for _, child := range cmd.Commands() {
		markAllGroupsAsStrict(child)
	}
}
