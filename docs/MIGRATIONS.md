# Orbit — Schema Migrations Plan

> **Status:** Draft (deferred — not implemented in M0)
> **Last updated:** 2026-06-23

## Why

`schema.sql` uses `CREATE TABLE IF NOT EXISTS`, so schema changes
are silently ignored on existing DBs. As a stop-gap, `db.Initialize`
stamps a SHA-256 fingerprint of `schema.sql` into `PRAGMA user_version`
and returns `ErrSchemaDrift` on mismatch. Resolution today is
`orbit destroy --yes && orbit init` — wipe and rebuild, **losing
all work entries, tags, logs, and other user data**. 

The goal of this plan is to upgrade the DB in place and the user's data
survives the upgrade.

## Schema diffs

We store **diffs** — small SQL files, each describing the change
from the previous version to the next. The current schema is
whatever you get by running every diff in order on an empty DB.

```
internal/db/
  migrations/
    0001_init.sql              # today's schema.sql, verbatim
                               # (CREATE TABLE work_entries, tags, state, ...)
    0002_add_priority.sql      # ALTER TABLE work_entries ADD COLUMN priority ...
    0003_unique_title.sql      # 12-step rewrite to add UNIQUE on title
    NNNN_short_description.sql
```

A `schema_migrations` table in the DB records which numbered files
have already been applied:

```sql
CREATE TABLE schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
```

On startup, `db.Migrate` reads this table, walks the embedded files
in order, and applies any whose number isn't already in the table.

This replaces both today's `schema.sql` and the `user_version`
fingerprint check.

## How user data survives

### Trivial migrations — just `ALTER`

SQLite preserves existing rows automatically. Each row gets the
default for the new column.

```sql
-- 0002_add_priority.sql
ALTER TABLE work_entries ADD COLUMN priority INTEGER NOT NULL DEFAULT 0;
```

### Non-trivial migrations — the 12-step rewrite

SQLite can't drop a column or add a constraint to an existing
table in place. The
[12-step procedure](https://sqlite.org/lang_altertable.html)
explicitly copies data through a new table:

```sql
-- 0003_unique_title.sql
CREATE TABLE work_entries_new (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL UNIQUE COLLATE NOCASE,  -- new constraint
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'new' COLLATE NOCASE,
    status_reason   TEXT,
    pad_path        TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

INSERT INTO work_entries_new
SELECT id, title, description, status, status_reason,
       pad_path, created_at, updated_at
  FROM work_entries;

DROP TABLE work_entries;
ALTER TABLE work_entries_new RENAME TO work_entries;

-- Recreate indexes that lived on the old table.
CREATE INDEX idx_work_entries_status     ON work_entries(status);
CREATE INDEX idx_work_entries_created_at ON work_entries(created_at);
```

Rows are physically copied into the new table. Same data, new
shape. The whole thing runs inside a transaction, so either every
step lands or none of them do.

### Data-transforming migrations

When the change is to the *values*, not the structure, the
migration owns the transform:

```sql
-- 0005_normalize_status.sql
UPDATE work_entries SET status = lower(status);
```

If a transform ever needs Go code (string parsing, time-zone math,
etc.), we'll add a parallel registry of `func(*sql.Tx) error`
keyed by version that runs after the file's SQL. Punt until needed
— SQL is enough for the foreseeable shape of orbit's data.

## `db.Migrate` — the runtime

Replaces today's `db.Initialize`:

1. Create `schema_migrations` if missing (bootstrap).
2. Read applied versions into a set.
3. If there's at least one unapplied migration, copy
   `orbit.db → orbit.db.bak-pre-NNNN` first. This is a
   belt-and-suspenders backup, not the primary mechanism — if a
   migration ships broken, the user can `mv` the backup over the
   live DB and downgrade. In the happy path it's never touched.
4. For each unapplied file in lexical order:
   - `BEGIN IMMEDIATE`
   - exec the file's SQL
   - `INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`
   - `COMMIT`

   On any error: `ROLLBACK` and return. The next run retries
   cleanly from the same point.
5. If `max(applied) > max(embedded)` → `ErrSchemaDrift`. This
   means the DB was written by a newer orbit binary than the one
   currently running. The DB and its data are untouched; the user
   needs to upgrade (or downgrade) the binary.

The other half of today's `ErrSchemaDrift` ("binary is newer than
DB") is no longer an error — it's the normal upgrade path, handled
by steps 3–4.

## Authoring conventions

- `NNNN_short_snake_case.sql`, zero-padded to 4 digits, gap-free.
- **Forward-only.** To revert a change, write a new migration.
- One logical change per file. Keeps review easy and lets us
  bisect bad migrations by number.
- Non-trivial table changes use the 12-step rewrite above.
- Test each migration by:
  - applying `0001..NNNN-1` to an empty DB,
  - seeding representative fixture rows,
  - applying `NNNN`,
  - asserting the post-state (rows preserved, new constraints
    enforced, indexes present).
- Plus a global test: applying all migrations to an empty DB
  produces the expected final schema (compared against a golden
  `sqlite3 .schema` dump).
- Plus an idempotency test: running `Migrate` twice in a row is a
  no-op the second time.

## What this plan does *not* cover

- **`orbit destroy`** is still destructive by design. Migrations
  preserve data across version bumps; `destroy` is the
  deliberately-unsafe escape hatch and stays that way.
- **Settings preservation across `destroy`** (the dock root and
  other `state` fields) is a separate, smaller piece of work
  involving the `config.yaml` file, tracked elsewhere.
- **Exporting user data** (for sync, sharing, off-machine backup)
  is its own feature — `orbit export` / `orbit import` — and is
  not what migrations are for.
