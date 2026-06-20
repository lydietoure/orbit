# Orbit — Schema Migrations Plan

> **Status:** Draft (deferred — not implemented in M0)
> **Last updated:** 2026-06-19

## Why

`schema.sql` uses `CREATE TABLE IF NOT EXISTS`, so schema changes
are silently ignored on existing DBs. As a stop-gap, `db.Initialize`
stamps a SHA-256 fingerprint of `schema.sql` into `PRAGMA user_version`
and returns `ErrSchemaDrift` on mismatch. Resolution today is
`orbit destroy --yes && orbit init` — wipe and rebuild. Fine until
the DB has data worth keeping.

## Plan

Hand-rolled numbered SQL migrations embedded in the binary. No
external library (`golang-migrate`, `goose`) — too much ceremony for
a single-user local SQLite app.

```
internal/db/
  migrations/
    0001_init.sql              # today's schema.sql, verbatim
    0002_unique_title.sql      # the ALTER we just added
    NNNN_short_description.sql
```

`db.Migrate(db)`:

1. Bootstrap `schema_migrations(version INTEGER PK, applied_at TEXT)`.
2. `SELECT version FROM schema_migrations` → applied set.
3. For each unapplied file in lexical order: `BEGIN`, exec, `INSERT`, `COMMIT`.
4. If max applied > max embedded → `ErrSchemaDrift` (DB is from a
   newer binary).

Replaces both `schema.sql` and the `user_version` fingerprint check.

## Conventions

- `NNNN_*.sql`, zero-padded, gap-free.
- Forward-only. To revert, write a new migration.
- One logical change per file.
- SQLite `ALTER TABLE` is limited; non-trivial changes use the
  [12-step table rewrite](https://sqlite.org/lang_altertable.html).

## When

Build it when **both** are true:
1. A real (non-throwaway) Orbit DB exists.
2. You want to ship a schema change.

Until then, drift detection + destroy/init is enough.

## Parked for later

- `orbit db status` (current vs. latest version)
- `--dry-run` for migrations
- Auto-backup `orbit.db` → `orbit.db.bak-NNNN` before applying
