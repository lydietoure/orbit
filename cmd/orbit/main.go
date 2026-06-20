package main

import (
	"os"

	"github.com/lydietoure/orbit/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		// Exit 2 (POSIX "incorrect usage") for command-line mistakes
		// like unknown subcommands or missing required subcommands;
		// exit 1 for everything else.
		if cli.AsUsageError(err) != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
