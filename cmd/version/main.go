// Command version prints the build version string for use in ldflags.
// It reads VERSION.txt and appends a git SHA suffix if HEAD is not a tag.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	base := readBase()
	meta := buildMeta()

	if meta != "" {
		fmt.Printf("%s+%s", base, meta)
	} else {
		fmt.Print(base)
	}
}

func readBase() string {
	data, err := os.ReadFile("VERSION.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading VERSION.txt: %v\n", err)
		os.Exit(1)
	}
	return strings.TrimSpace(string(data))
}

func buildMeta() string {
	// If HEAD is an exact tag, no suffix needed.
	cmd := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD")
	if err := cmd.Run(); err == nil {
		return ""
	}

	// Get short SHA
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(out))

	// Check if tree is dirty
	if err := exec.Command("git", "diff", "--quiet").Run(); err != nil {
		sha += "-dirty"
	}

	return sha
}
