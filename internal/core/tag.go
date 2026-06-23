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

// TagCount pairs a tag name with the number of work entries that
// carry it. It is the read-model behind `orbit tags`, which lists the
// tag vocabulary with per-tag usage counts.
type TagCount struct {
	Name  string
	Count int
}

// Reserved tag prefixes recognised at the application layer. They are
// ordinary tags in the database; the cardinality rules in
// docs/DATA_MODEL.md (`project:*` multiple, `owner:*` single) are
// enforced by the app package, not the schema.
const (
	// ProjectTagPrefix marks a tag as a project association.
	ProjectTagPrefix = "project:"
	// OwnerTagPrefix marks a tag as the owning context.
	OwnerTagPrefix = "owner:"
)

// ProjectTagName builds the canonical `project:<name>` tag from a bare
// project name. The name is run through [NormalizeTagName]; an already
// `project:`-prefixed value is accepted and not double-prefixed. The
// bare name must be non-empty after the prefix is stripped.
func ProjectTagName(raw string) (string, error) {
	return reservedTagName(ProjectTagPrefix, "project", raw)
}

// OwnerTagName builds the canonical `owner:<name>` tag from a bare
// owner name. Same normalization and prefix-tolerance rules as
// [ProjectTagName].
func OwnerTagName(raw string) (string, error) {
	return reservedTagName(OwnerTagPrefix, "owner", raw)
}

// reservedTagName normalizes raw, strips an optional leading prefix so
// callers may pass either `payments` or `project:payments`, and
// returns prefix+bare. kind is used only for error messages.
func reservedTagName(prefix, kind, raw string) (string, error) {
	name, err := NormalizeTagName(raw)
	if err != nil {
		return "", err
	}
	bare := strings.TrimPrefix(name, prefix)
	if bare == "" {
		return "", fmt.Errorf("%s name is required", kind)
	}
	return prefix + bare, nil
}

// IsReservedTag reports whether name uses one of the reserved tag
// conventions (`project:*` or `owner:*`). Generic tag displays such as
// `orbit tags` use it to hide reserved tags, which have their own
// dedicated `work project` / `work owner` views.
func IsReservedTag(name string) bool {
	return strings.HasPrefix(name, ProjectTagPrefix) ||
		strings.HasPrefix(name, OwnerTagPrefix)
}

// PartitionReservedTags splits a sorted tag list into the bare project
// names, the bare owner name, and the remaining plain tags. The
// `project:`/`owner:` prefixes are removed from the project and owner
// results. owner is "" when no `owner:*` tag is present; if more than
// one is somehow present (the app layer prevents this) the first in
// sorted order wins. Input order is preserved within each group.
func PartitionReservedTags(tags []string) (projects []string, owner string, plain []string) {
	for _, t := range tags {
		switch {
		case strings.HasPrefix(t, ProjectTagPrefix):
			projects = append(projects, strings.TrimPrefix(t, ProjectTagPrefix))
		case strings.HasPrefix(t, OwnerTagPrefix):
			if owner == "" {
				owner = strings.TrimPrefix(t, OwnerTagPrefix)
			}
		default:
			plain = append(plain, t)
		}
	}
	return projects, owner, plain
}
