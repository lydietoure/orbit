package core

import (
	"math/rand/v2"
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
	// StatusCompleted means the entry is finished.
	StatusCompleted WorkEntryStatus = "completed"
	// StatusAbandoned means the entry was dropped without completion;
	// a reason is required to record why.
	StatusAbandoned WorkEntryStatus = "abandoned"
)

// Valid reports whether s is one of the known status values.
func (s WorkEntryStatus) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusCompleted, StatusAbandoned:
		return true
	}
	return false
}

// WorkEntry is a single unit of tracked work. The ID is a 5-character
// Crockford base32 string; see [NewID] and docs/DATA_MODEL.md for the
// encoding scheme. The full set of persisted fields (description,
// status reason, scratchpad path, timestamps) is added in later
// slices as the storage layer needs them.
type WorkEntry struct {
	ID     string
	Title  string
	Status WorkEntryStatus
}

// ArtifactType classifies an [Artifact] (e.g., a git branch, a pull
// request URL, a repository path). The set of supported types is
// defined by the application layer, not the database.
type ArtifactType string

// Artifact is a reference attached to a [WorkEntry] that points at
// something living outside Orbit (a branch, PR, repo, etc.). Orbit
// does not store the content of the artifact, only the reference.
type Artifact struct {
	ID          int64
	WorkEntryID string
	Type        ArtifactType
	Value       string
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
