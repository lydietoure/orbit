package core

// Domain types for Orbit. See docs/DATA_MODEL.md for details.

type WorkEntryStatus string

type WorkEntry struct {
	ID     string
	Title  string
	Status WorkEntryStatus
}

type ArtifactType string

type Artifact struct {
	ID          int64
	WorkEntryID string
	Type        ArtifactType
	Value       string
}

type Tag struct {
	ID   int64
	Name string
}
