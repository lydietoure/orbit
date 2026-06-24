-- Orbit database schema (M0)
--
-- See docs/DATA_MODEL.md for the conceptual model and docs/TECH_STACK.md
-- for implementation details. This file is embedded into the binary via
-- //go:embed and applied on every DB open. Statements use
-- CREATE TABLE IF NOT EXISTS so re-applying is a no-op on existing DBs.
--
-- Pragmas (foreign_keys, journal_mode) are set on the connection, not
-- here, so they apply to every open.

-- TODO: Remove `IF NOT EXISTS` from all statements once we have a migration system in place.

-- Work entries: the central entity.
--
-- title carries UNIQUE COLLATE NOCASE so duplicate titles are
-- rejected at the storage layer regardless of casing ("Foo" and
-- "foo" collide). Enforcing in the schema keeps the check
-- race-safe; an app-level pre-query would have a TOCTOU window.
CREATE TABLE IF NOT EXISTS work_entries (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL UNIQUE COLLATE NOCASE,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'new' COLLATE NOCASE,
    status_reason   TEXT,
    pad_path        TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_work_entries_status     ON work_entries(status);
CREATE INDEX IF NOT EXISTS idx_work_entries_created_at ON work_entries(created_at);

-- Tags: free-form labels (including the project:* and owner:* conventions).
-- The UNIQUE constraint on `name` implicitly creates a lookup index.
CREATE TABLE IF NOT EXISTS tags (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

-- Many-to-many join between work entries and tags.
CREATE TABLE IF NOT EXISTS work_entry_tags (
    work_entry_id TEXT    NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    tag_id        INTEGER NOT NULL REFERENCES tags(id)         ON DELETE CASCADE,
    PRIMARY KEY (work_entry_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_work_entry_tags_tag_id ON work_entry_tags(tag_id);

-- Application state: a singleton row with id=1. Tracks the currently
-- selected work entry (auto-cleared if it is deleted), the timestamp
-- of the last lazy health check, and the user's "dock" preferences
-- (where pads live, and whether they're auto-provisioned).
--
-- dock_root NULL means "no dock configured"; the ORBIT_DOCK env var
-- overrides this value at read time.
-- dock_auto_create is stored as 0/1 (SQLite has no native bool) with
-- a CHECK so accidental writes of other ints fail loudly.
CREATE TABLE IF NOT EXISTS state (
    id                     INTEGER PRIMARY KEY CHECK (id = 1),
    selected_work_entry_id TEXT REFERENCES work_entries(id) ON DELETE SET NULL,
    last_health_check      TEXT,
    dock_root              TEXT,
    dock_auto_create       INTEGER NOT NULL DEFAULT 0
                           CHECK (dock_auto_create IN (0, 1))
);

INSERT OR IGNORE INTO state (id) VALUES (1);
