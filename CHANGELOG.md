# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `orbit work status [id] <status>` — set a work entry's status
  (`new`, `in-progress`, `completed`, `abandoned`). `--reason` is
  required for `abandoned` and optional otherwise; clearing it removes
  any prior reason. Every transition is allowed, but moving backward
  along the lifecycle prints a warning. The id is optional and falls
  back to the selected entry.
- `orbit work close [id]` — shortcut that completes a work entry, or
  with `--abandon --reason` marks it abandoned. The id is optional and
  falls back to the selected entry.
- `orbit work project add|remove|list [id] [name]` — manage the
  `project:*` tags on a work entry. Projects are multi-valued and
  adding is idempotent. The id is optional and falls back to the
  selected entry.
- `orbit work owner add|remove|list [id] [name]` — manage the single
  `owner:*` tag on a work entry. `add` sets the owner (replacing any
  existing one), `remove` clears it, and `list` shows it. The id is
  optional and falls back to the selected entry.
- `orbit work show`, `work list`, and `work selected` now surface
  owner and project tags distinctly from plain tags.

## [0.1.0] - 2026-06-19

First published release. Orbit was previously prototyped in Python
(tagged `python/final`); 0.1.0 is the first cut of the Go rewrite
and is not data-compatible with that prototype.

This release is the foundation: tracked work entries with a current
focus, per-entry "pad" folders, and a scriptable CLI. Linking,
logging, and reporting are deferred to later milestones — see
[docs/DESIGN.md](docs/DESIGN.md).

### Added

#### Lifecycle and configuration

- `orbit init` — create `~/.orbit/` with `config` and `orbit.db`.
  Safe to re-run; `--dry-run` reports what would change.
- `orbit destroy` — remove the orbit home. `--yes` skips the
  confirmation prompt; `--dry-run` reports what would be deleted.
- `orbit config dock get|set|unset` — persist the dock root, the
  directory under which new pads are provisioned. `get --raw` prints
  the bare path for scripting. `ORBIT_DOCK` overrides the stored value.
- `orbit config dock auto-create <bool>` — control whether the dock
  root is auto-created on demand.

#### Work entries

- `orbit work new "<title>"` — create an entry and auto-select it.
  Flags: `-p/--pad <name>` (provision a pad folder), `--no-dock`
  (force CWD even when a dock root is set), `--no-select` (skip
  auto-selection), `-t/--tag <tag>` (repeatable).
- `orbit work list` — list entries.
- `orbit work show [id]` — details; defaults to the selected entry.
- `orbit work select <id>` / `orbit work selected` /
  `orbit work forget` — manage the current focus.
- `orbit work tag [id] <tag>` — add a tag. `--remove` drops it.
  The id is optional and falls back to the selected entry.
- `orbit work delete [id]` — delete an entry. `--yes` skips the
  prompt; `--purge` also removes the pad folder from disk.

#### Pad management

- `orbit work pad show [id]` — print the pad path and whether the
  directory exists.
- `orbit work pad get [id]` — print the bare pad path (scripting).
- `orbit work pad set [id] <path>` — attach or move a pad. `--no-dock`
  pins the path to CWD when relative. When the target directory
  already exists it is reused as-is, with a one-line note on stderr.
- `orbit work pad clear [id]` — clear the entry's pad column. Does
  not touch disk.

#### Cross-cutting

- POSIX exit codes: `0` success, `1` runtime error, `2` incorrect
  usage. Bare command groups and unknown subcommands exit `2` with
  a usage error instead of silently printing help.
- Global `--verbose` / `--debug` flags; equivalent `ORBIT_VERBOSE`
  and `ORBIT_DEBUG` environment variables. Structured `slog` output
  to stderr.
- Local-first SQLite persistence via pure-Go `modernc.org/sqlite`.
  Schema migrations gated by `PRAGMA user_version`; foreign-key
  cascades clear the selection pointer and tag links when an entry
  is deleted.
- Build-time version injection via ldflags; `orbit --version`
  reports either the release tag (release builds) or `dev`
  (developer builds).
- Smoke-test harness (`task smoke`) and convenience runner
  (`task run`) that target a throwaway and the default orbit home
  respectively.

#### Documentation

- Design document, data model, tech stack, and development guide
  under `docs/`.
- README describing the 0.1.0 surface and pointing to the roadmap.
