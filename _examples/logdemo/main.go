// logdemo emits sample log lines through orbit's diag package so you can
// see what each verbosity level looks like in a real terminal.
//
// Usage:
//
//	go run ./_examples/logdemo            # default (warn+)
//	go run ./_examples/logdemo -verbose   # info+
//	go run ./_examples/logdemo -debug     # debug+ (with timestamps)
//
// This file lives under _examples/ so it is skipped by `go build ./...`
// and `go test ./...` (Go tooling ignores directories whose name starts
// with `_` or `.`). Run it explicitly when you want to eyeball the
// formatter — e.g. after changing diag.Setup.
package main

import (
	"flag"
	"log/slog"

	"github.com/lydietoure/orbit/internal/diag"
)

func main() {
	verbose := flag.Bool("verbose", false, "info-level logging")
	debug := flag.Bool("debug", false, "debug-level logging (implies -verbose, adds timestamps)")
	flag.Parse()

	diag.Setup(*verbose, *debug)

	slog.Debug("resolved scratchpad path",
		"name", "payments-caching",
		"abs", "C:/scratch/payments-caching",
		"from_root", true,
	)
	slog.Info("created work entry",
		"id", "w-3a7f",
		"title", "Add caching to payment flow",
	)
	slog.Warn("scratchpad path already exists",
		"path", "C:/scratch/payments-caching",
	)
	slog.Error("failed to open db",
		"path", "~/.orbit/orbit.db",
		"err", "permission denied",
	)
}
