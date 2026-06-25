package core

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ArtifactType classifies an [Artifact] (e.g., a git branch, a pull
// request URL, a repository path). See docs/DATA_MODEL.md for what
// each type means. Values are stored as lowercase strings so they
// round-trip cleanly through the database.
type ArtifactType string

const (
	// ArtifactBranch is a git branch name.
	ArtifactBranch ArtifactType = "branch"
	// ArtifactPR is a pull-request URL.
	ArtifactPR ArtifactType = "pr"
	// ArtifactWorkItem is an issue or work-item URL (ADO, GitHub, ...).
	ArtifactWorkItem ArtifactType = "workitem"
	// ArtifactRepo is the local path to a git repository.
	ArtifactRepo ArtifactType = "repo"
	// ArtifactDir is the local path to a directory (non-repo).
	ArtifactDir ArtifactType = "dir"
	// ArtifactFile is the local path to a file.
	ArtifactFile ArtifactType = "file"
	// ArtifactURL is any other URL (docs, wiki, ...).
	ArtifactURL ArtifactType = "url"
	// ArtifactNote is the local path to a markdown file the user
	// maintains elsewhere (an Obsidian vault, a project folder, ...).
	ArtifactNote ArtifactType = "note"
	// ArtifactCustom is a user-defined freeform reference.
	ArtifactCustom ArtifactType = "custom"
)

// Artifact is a reference attached to a [WorkEntry] that points at
// something living outside Orbit (a branch, PR, repo, etc.). Orbit
// does not store the content of the artifact, only the reference.
type Artifact struct {
	ID          int64
	WorkEntryID string
	Type        ArtifactType
	Value       string
	CreatedAt   time.Time
}

// Valid reports whether t is one of the known artifact types.
func (t ArtifactType) Valid() bool {
	switch t {
	case ArtifactBranch, ArtifactPR, ArtifactWorkItem, ArtifactRepo,
		ArtifactDir, ArtifactFile, ArtifactURL, ArtifactNote, ArtifactCustom:
		return true
	}
	return false
}

// IsLocalPath reports whether t references a path on the local
// filesystem (repo, dir, file, note). Local-path artifacts are stored
// as absolute paths; the absolutization happens in the app layer since
// it depends on the working directory.
func (t ArtifactType) IsLocalPath() bool {
	switch t {
	case ArtifactRepo, ArtifactDir, ArtifactFile, ArtifactNote:
		return true
	}
	return false
}

// IsURL reports whether t references a URL (pr, workitem, url). These
// types reject obviously-invalid values via [ArtifactType.NormalizeValue].
func (t ArtifactType) IsURL() bool {
	switch t {
	case ArtifactPR, ArtifactWorkItem, ArtifactURL:
		return true
	}
	return false
}

// ParseArtifactType normalizes raw (trim + lowercase) and returns the
// matching [ArtifactType], or an error naming the offending value.
func ParseArtifactType(raw string) (ArtifactType, error) {
	t := ArtifactType(strings.ToLower(strings.TrimSpace(raw)))
	if !t.Valid() {
		return "", fmt.Errorf("unknown artifact type %q", raw)
	}
	return t, nil
}

// NormalizeValue validates and canonicalizes an artifact value for
// this type, returning the form to store. The rules are:
//
//   - The value is trimmed; an empty result is rejected.
//   - URL types (pr, workitem, url) must parse as an absolute URL with
//     a scheme and host — a cheap guard against obvious typos. Other
//     schemes than http(s) are allowed (e.g. ssh:// remotes).
//   - Local-path types (repo, dir, file) are returned trimmed but NOT
//     absolutized — that is the app layer's job (it needs the working
//     directory). The path is otherwise unconstrained; Orbit only
//     references files, so a missing path is a warning, not an error.
//   - branch and custom are accepted as-is once non-empty.
func (t ArtifactType) NormalizeValue(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("a value is required to link a %s artifact", t)
	}
	if t.IsURL() && !looksLikeURL(value) {
		return "", fmt.Errorf("%q is not a valid URL for a %s artifact", value, t)
	}
	return value, nil
}

// looksLikeURL reports whether raw parses as an absolute URL with both
// a scheme and a host. Deliberately permissive about the scheme so
// non-http references (git@, ssh://, etc.) still pass — the goal is to
// catch obvious mistakes like a bare branch name passed as a URL, not
// to enforce a particular protocol.
func looksLikeURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}
