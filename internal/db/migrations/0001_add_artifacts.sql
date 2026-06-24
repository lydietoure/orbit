-- Orbit database schema (M0)
--
-- Adds the artifacts and notes tables (and their indexes) introduced
-- after v0.1.0.


-- Artifacts: typed references attached to a work entry (a branch, PR,
-- repo path, URL, etc.). Orbit only references these things — it never
-- owns or creates them. `type` is one of the ArtifactType values
-- validated in the core layer; the schema deliberately stays agnostic
-- so adding a type later needs no migration.
--
-- The UNIQUE(work_entry_id, type, value) constraint makes re-linking
-- the same reference idempotent (INSERT OR IGNORE) and gives `remove`
-- a deterministic target. ON DELETE CASCADE ties an artifact's life to
-- its parent entry.
CREATE TABLE artifacts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    work_entry_id TEXT NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    type          TEXT NOT NULL,
    value         TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    UNIQUE (work_entry_id, type, value)
);

CREATE INDEX idx_artifacts_work_entry_id ON artifacts(work_entry_id);

-- Notes: dated references to markdown files the user maintains
-- elsewhere (an Obsidian vault, a project folder, ...). Orbit stores
-- the absolute path and a logical date; it never creates or owns the
-- file. `date` is the logical YYYY-MM-DD the note belongs to and feeds
-- work-day tracking later.
--
-- UNIQUE(work_entry_id, path, date) keeps re-linking the same note on
-- the same date idempotent while still allowing the same file to be
-- referenced on different dates. ON DELETE CASCADE ties a note to its
-- parent entry.
CREATE TABLE notes (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    work_entry_id TEXT NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    path          TEXT NOT NULL,
    date          TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    UNIQUE (work_entry_id, path, date)
);

CREATE INDEX  idx_notes_work_entry_id ON notes(work_entry_id);