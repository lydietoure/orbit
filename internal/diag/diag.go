// Package diag handles application-wide diagnostics: structured logging
// and (eventually) user-facing error reporting and exit-code mapping.
//
// Orbit uses log/slog as the logging API throughout the codebase. This
// package centralizes handler configuration so call sites stay
// handler-agnostic — swapping in a different slog.Handler (e.g. a JSON
// handler in CI) is a one-line change here.
//
// Today the package only contains the logger setup. Error helpers
// (sentinels, exit-code mapping, Report) will be added here when the
// first command needs them.
package diag

import (
	"log/slog"
	"os"

	charmlog "github.com/charmbracelet/log"
)

// Setup configures the default slog logger based on the --verbose / --debug
// flags. Logs go to stderr; stdout is reserved for command output.
//
// Level mapping:
//
//	default         → Warn  (errors and warnings only)
//	--verbose / -v  → Info
//	--debug         → Debug (implies --verbose)
//
// Timestamps are hidden by default for a clean interactive experience and
// re-enabled with --debug so post-mortem reconstruction is possible.
func Setup(verbose, debug bool) {
	level := charmlog.WarnLevel
	switch {
	case debug:
		level = charmlog.DebugLevel
	case verbose:
		level = charmlog.InfoLevel
	}

	handler := charmlog.NewWithOptions(os.Stderr, charmlog.Options{
		Level:           level,
		ReportTimestamp: debug,
		TimeFormat:      "15:04:05.000",
	})
	slog.SetDefault(slog.New(handler))
}
