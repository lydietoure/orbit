package core

import (
	"errors"
	"fmt"
	"strings"
)

// NormalizeTagName produces the canonical form of a tag name as it
// will be stored in the database. The rules — kept deliberately
// loose for free-form labels — are:
//
//   - Trim surrounding whitespace.
//   - Lower-case (so "Caching" and "caching" are the same tag,
//     matching the convention used throughout docs/DESIGN.md).
//   - Reject empty results.
//   - Reject names containing ',' — cobra's StringSliceVar splits
//     `--tag a,b` into two tags, so a literal comma in a tag name
//     would be lost in the CLI round-trip.
//
// Internal whitespace and punctuation otherwise (including the
// reserved prefixes `project:` and `owner:`) are allowed. Stricter
// validation can be layered on later without breaking storage.
func NormalizeTagName(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return "", errors.New("tag name is required")
	}
	if strings.ContainsRune(name, ',') {
		return "", fmt.Errorf("tag %q must not contain a comma", raw)
	}
	return name, nil
}
