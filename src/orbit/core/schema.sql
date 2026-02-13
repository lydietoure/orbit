-- Orbit database schema
-- See docs/DATA_MODEL.md for conceptual model
-- See docs/TECH_STACK.md for implementation details

-- Work entries: the central entity
CREATE TABLE IF NOT EXISTS work_entries (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'new' COLLATE NOCASE,
    status_reason   TEXT,
    scratchpad_path TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_work_entries_status ON work_entries(status);
CREATE INDEX IF NOT EXISTS idx_work_entries_created_at ON work_entries(created_at);

-- Tags: free-form labels (including project:* and owner:* conventions)
CREATE TABLE IF NOT EXISTS tags (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    name    TEXT NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);

-- Many-to-many join between work entries and tags
CREATE TABLE IF NOT EXISTS work_entry_tags (
    work_entry_id   TEXT NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    tag_id          INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (work_entry_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_work_entry_tags_tag_id ON work_entry_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_work_entry_tags_work_entry_id ON work_entry_tags(work_entry_id);

-- Artifacts: linked references (branches, PRs, repos, files, URLs, etc.)
CREATE TABLE IF NOT EXISTS artifacts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    work_entry_id   TEXT NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    type            TEXT NOT NULL,
    value           TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_artifacts_work_entry_id ON artifacts(work_entry_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_type ON artifacts(type);

-- Notes: dated references to user-managed markdown files
CREATE TABLE IF NOT EXISTS notes (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    work_entry_id   TEXT NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    path            TEXT NOT NULL,
    date            TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notes_work_entry_id ON notes(work_entry_id);
CREATE INDEX IF NOT EXISTS idx_notes_date ON notes(date);

-- Log entries: timestamped one-liners
CREATE TABLE IF NOT EXISTS log_entries (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    work_entry_id   TEXT NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    message         TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_log_entries_work_entry_id ON log_entries(work_entry_id);
CREATE INDEX IF NOT EXISTS idx_log_entries_created_at ON log_entries(created_at);

-- Work days: dates on which work occurred
CREATE TABLE IF NOT EXISTS work_days (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    work_entry_id   TEXT NOT NULL REFERENCES work_entries(id) ON DELETE CASCADE,
    date            TEXT NOT NULL,
    source          TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    UNIQUE(work_entry_id, date)
);

CREATE INDEX IF NOT EXISTS idx_work_days_work_entry_id ON work_days(work_entry_id);
CREATE INDEX IF NOT EXISTS idx_work_days_date ON work_days(date);

-- Application state: singleton (always id=1)
CREATE TABLE IF NOT EXISTS state (
    id                      INTEGER PRIMARY KEY CHECK (id = 1),
    selected_work_entry_id  TEXT REFERENCES work_entries(id) ON DELETE SET NULL,
    last_health_check       TEXT
);

-- Seed the singleton state row
INSERT OR IGNORE INTO state (id) VALUES (1);
