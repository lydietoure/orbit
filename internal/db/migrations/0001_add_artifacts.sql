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
CREATE TABLE IF NOT EXISTS artifacts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    work_entry_id TEXT NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    type          TEXT NOT NULL,
    value         TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    UNIQUE (work_entry_id, type, value)
);

CREATE INDEX IF NOT EXISTS idx_artifacts_work_entry_id ON artifacts(work_entry_id);