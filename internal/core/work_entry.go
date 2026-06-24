package core

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

// Domain types for Orbit. See docs/DATA_MODEL.md for details.

// WorkEntryStatus is the lifecycle status of a WorkEntry. Values are
// stored as lowercase strings so they round-trip cleanly through the
// database and YAML exports.
type WorkEntryStatus string

const (
	// StatusNew means the entry has been created but no work has started.
	StatusNew WorkEntryStatus = "new"
	// StatusInProgress means the entry is actively being worked on.
	StatusInProgress WorkEntryStatus = "in-progress"
	// StatusPaused means work has started but is temporarily on hold.
	// It is lateral to in-progress: pausing and resuming move between
	// the two without counting as progress or a step backward.
	StatusPaused WorkEntryStatus = "paused"
	// StatusCompleted means the entry is finished.
	StatusCompleted WorkEntryStatus = "completed"
	// StatusAbandoned means the entry was dropped without completion;
	// a reason is required to record why.
	StatusAbandoned WorkEntryStatus = "abandoned"
)

// Valid reports whether s is one of the known status values.
func (s WorkEntryStatus) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusPaused, StatusCompleted, StatusAbandoned:
		return true
	}
	return false
}

// rank places a status on the lifecycle line so transitions can be
// compared: new < {in-progress, paused} < {completed, abandoned}.
// in-progress and paused share a rank — pausing and resuming are
// lateral moves, not progress or regress — as do the two terminal
// states. Unknown statuses rank below everything so a transition away
// from them never reads as "backward".
func (s WorkEntryStatus) rank() int {
	switch s {
	case StatusNew:
		return 1
	case StatusInProgress, StatusPaused:
		return 2
	case StatusCompleted, StatusAbandoned:
		return 3
	}
	return 0
}

// IsBackwardTransition reports whether moving from one status to another
// is a step backward along the lifecycle (e.g. completed → in-progress).
// It is purely informational: every transition is permitted, but callers
// may warn the user when this returns true. Re-stating the same status is
// not backward.
func IsBackwardTransition(from, to WorkEntryStatus) bool {
	return to.rank() < from.rank()
}

// WorkEntry is a single unit of tracked work. The ID is a 5-character
// Crockford base32 string; see [NewID] and docs/DATA_MODEL.md for the
// encoding scheme.
//
// Optional text fields (Description, StatusReason, PadPath)
// use the empty string to mean "absent" — the storage layer maps
// empty to SQL NULL on write.
type WorkEntry struct {
	ID           string
	Title        string
	Description  string
	Status       WorkEntryStatus
	StatusReason string
	PadPath      string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// Tags is the alphabetically-sorted list of tag names attached
	// to this entry. Populated by the storage layer on read; left
	// nil by [NewWorkEntry] (tags are applied after insert).
	Tags []string
	// Artifacts is the list of typed references linked to this entry,
	// oldest first. Populated by the storage layer on read (see
	// [github.com/lydietoure/orbit/internal/db.GetWorkEntry]); left
	// nil by [NewWorkEntry].
	Artifacts []Artifact
}

// NewWorkEntryParams holds the user-supplied input for a new work
// entry. Computed fields (ID, timestamps) are filled in by
// [NewWorkEntry], not the caller.
type NewWorkEntryParams struct {
	// Title is required and is trimmed of surrounding whitespace.
	Title string
	// Description is an optional longer explanation. Empty means absent.
	Description string
	// Status is the initial lifecycle status. Defaults to [StatusNew]
	// when zero.
	Status WorkEntryStatus
	// StatusReason explains the status. Required when Status is
	// [StatusAbandoned]; otherwise optional.
	StatusReason string
	// PadPath is an optional filesystem path to the pad — the
	// per-entry folder for experimental/scratch work. Empty means absent.
	PadPath string
}

// NewWorkEntry builds a fully-populated, validated [WorkEntry]. It
// trims the title, defaults Status to [StatusNew] when zero,
// enforces the data-model rule that an abandoned entry requires a
// reason, generates a fresh ID via [NewID], and sets CreatedAt and
// UpdatedAt to the current time (UTC, equal at construction).
//
// The returned WorkEntry is ready to persist; storage layers should
// not re-validate or re-default.
func NewWorkEntry(p NewWorkEntryParams) (WorkEntry, error) {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		return WorkEntry{}, errors.New("work entry title is required")
	}

	// Title uniqueness (case-insensitive) is enforced by the DB layer
	// via a UNIQUE COLLATE NOCASE constraint on work_entries.title.
	// Checking here would require I/O and would race with concurrent
	// inserts; let the storage layer be the source of truth.

	status := p.Status
	if status == "" {
		status = StatusNew
	}
	if !status.Valid() {
		return WorkEntry{}, fmt.Errorf("invalid work entry status %q", status)
	}
	if status == StatusAbandoned && strings.TrimSpace(p.StatusReason) == "" {
		return WorkEntry{}, errors.New("status reason is required when status is abandoned")
	}

	now := time.Now().UTC()
	return WorkEntry{
		ID:           NewID(),
		Title:        title,
		Description:  p.Description,
		Status:       status,
		StatusReason: p.StatusReason,
		PadPath:      p.PadPath,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Tag is a free-form label that can be attached to many [WorkEntry]
// records via a join table. Tags with the prefixes `project:` and
// `owner:` are recognised at the application layer for ergonomic
// commands; in the database they are just tags.
type Tag struct {
	ID   int64
	Name string
}

// ---- ID generation ---------------------------------------------------------
//
// WorkEntry IDs are 5-character strings using a Crockford-style base32
// alphabet (digits + lowercase letters, skipping the visually ambiguous
// i, l, o, u). The full ID is 25 bits, laid out as:
//
//   bits 10..24 (3 most-significant chars): hours since idEpoch
//   bits  0.. 9 (2 least-significant chars): random entropy
//
// See docs/DATA_MODEL.md#id-generation for the rationale.

// idEpoch is the reference time for the time-prefix component of an ID.
// The 15-bit time field wraps about 3.7 years after this point.
var idEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// idAlphabet is Crockford base32: 0-9 then a-z minus i, l, o, u.
const idAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

const (
	idTimeBits = 15
	idRandBits = 10
	idTimeMask = (1 << idTimeBits) - 1 // 0x7FFF
	idRandMask = (1 << idRandBits) - 1 // 0x3FF
)

// NewID returns a new 5-character WorkEntry ID using the current time
// and a non-cryptographic random source.
func NewID() string {
	return NewIDAt(time.Now(), rand.Uint32())
}

// NewIDAt builds an ID for the given time using the given random bits
// (only the low idRandBits are used). Exposed for deterministic tests.
//
// Times before idEpoch produce IDs with an undefined (wrapped) time
// prefix and so should not be relied on for chronological ordering;
// this is acceptable since Orbit has no pre-2026 history to represent.
func NewIDAt(now time.Time, randBits uint32) string {
	hours := uint32(now.Sub(idEpoch).Hours())
	timePart := uint64(hours&idTimeMask) << idRandBits
	randPart := uint64(randBits & idRandMask)
	bits := timePart | randPart

	out := make([]byte, 5)
	for i := 4; i >= 0; i-- {
		out[i] = idAlphabet[bits&0x1F]
		bits >>= 5
	}
	return string(out)
}
