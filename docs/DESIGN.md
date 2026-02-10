# Orbit — Design Document


## 1. Problem

As a developer, the artifacts of my work are scattered:

- **Git branches** live in local repos and remotes
- **Work items** live in Azure DevOps or GitHub Issues
- **Pull requests** live on the hosting platform
- **Notes and learnings** live in markdown files, scratch pads, or my head
- **Docs and design files** live in project folders, wikis, or shared drives

There is no single place to answer: *"What was I working on for project X? What did I learn? Where are the artifacts?"*

Existing tools (ADO boards, GitHub Projects, Notion, Obsidian) each cover a slice, but none of them **unify the developer's view across all of these** in a lightweight, CLI-friendly, local-first way.

## 2. Vision

**Orbit** is a personal developer work tracker that maps your entire work universe — branches, PRs, work items, notes, learnings — into a single queryable graph. It is:

- **Local-first:** Your data lives on your machine (with optional sync).
- **CLI-first:** Fast, scriptable, composable with your existing workflow.
- **LLM-ready:** An MCP server lets you ask an AI *"What did I learn about caching during the payments project?"*

## 3. Goals and Non-Goals

### Goals

- Track **work entries** (a unit of work: a feature, a bug, a spike) and link them to their artifacts
- Link a work entry to: git branches, PRs, ADO/GitHub issues, local folders, markdown notes
- Capture **learnings and notes** as first-class entities attached to work entries
- Query work entries by date range, project, tags, artifact type
- Generate summaries for standups, retros, and performance reviews
- Expose data via an **MCP server** so an LLM can answer questions about your work history

### Non-Goals (for now)

- Team-wide tracking or dashboards — this is a **personal** tool
- Real-time sync with ADO/GitHub (pull-based, not webhook-driven)
- GUI or web UI (CLI and MCP only)
- Replacing your task board — Orbit supplements it, not replaces it
- Time tracking or session logging
- Writing back to external platforms (GitHub, ADO) — Orbit is read-only toward them

## 4. Core Concepts (Domain Model)

| Concept | Description |
|---|---|
| **WorkEntry** | The central unit. Represents a piece of work (feature, bug, spike, learning). Has a title, **description**, status (with reason), optional **scratchpad** path, tags, timestamps. The description is stored in the DB so the work entry remains self-explanatory even if all linked references become stale. |
| **Artifact** | Something linked to a WorkEntry. Types: `branch`, `pr`, `workitem`, `repo`, `file`, `url`, `custom`. |
| **Note** | A dated reference to a markdown file the user manages. Orbit does not own note storage — the user decides where notes live (Obsidian vault, a project folder, anywhere). Orbit tracks the path and the date. Notes may contain rich markdown (code blocks, images, links). |
| **LogEntry** | A timestamped one-liner attached to a WorkEntry, stored directly in the DB. Captures quick observations in the moment (from the terminal) without switching to a notes app. Lightweight complement to Notes — useful for timeline reconstruction, MCP search, and memory. |
| **WorkDay** | A date on which you worked on a WorkEntry. Acts as an index into your daily notes — orbit doesn't copy content from your journal, it just knows *which days* you worked on something, so you can go back to those daily notes yourself. |
| **Scratchpad** | An optional folder path where you do experimental work for this entry (test files, scratch code, prototypes). Unlike artifacts which are references, the scratchpad is where you actively work. One per WorkEntry. |
| **Tag** | A free-form label for cross-cutting concerns (e.g., `caching`, `perf`, `debugging`). |

### Status lifecycle

A WorkEntry has a **status** and an optional **reason** (free text explaining why it's in that state).

| Status | Default reason | Meaning |
|---|---|---|
| `new` | *(none)* | Just created, not yet started |
| `in-progress` | *(none)* | Actively being worked on |
| `completed` | *(none)* | Done |
| `abandoned` | *(required)* | Dropped — reason explains why (e.g., "descoped", "superseded by #w-8b2c") |

- `orbit work new` sets status to `new`.
- `orbit work close` sets status to `completed` (or `abandoned --reason "..."`)
- `orbit work status <id> <status>` sets any status explicitly, with an optional `--reason`.
- Reason defaults are sensible (empty for happy-path transitions), but `abandoned` requires a reason so you remember *why* you dropped it.

### Selected work entry

At any time, one WorkEntry can be **selected** as the current focus. This is stored in the DB (not per-terminal — it's global). When a work entry is selected:
- `orbit link` commands can omit the `<id>` — they default to the selected entry.
- `orbit status` highlights the selected entry.
- `orbit work show` with no args shows the selected entry.

```
orbit work select <id>              # Set the selected work entry
orbit work forget                   # Clear the selection (no entry is selected)
```

This is ergonomic sugar — every command still accepts an explicit `<id>`.

### Work days (diary)

A WorkEntry accumulates **work days** — dates on which you worked on it. Orbit doesn't extract or copy content from your daily notes; it builds an index of days so you can navigate back to your own journal.

Work days can be recorded:

- **Automatically** — orbit infers you worked on it today if any of these happened:
  - You linked a new note dated today
  - You added a log entry today
  - You linked a new artifact today
  - A note already linked to this entry was modified today (file mtime check, during lazy daily health check)

- **Manually** — `orbit work today` marks that you worked on the selected entry today, for when your work didn't produce any orbit-visible event.

The diary view assembles a chronological timeline of work days, annotated with what orbit knows happened each day:

```
$ orbit work diary w-3a7f
Work Entry: Add caching to payment flow (w-3a7f)

  2026-01-20  (log: "set up redis cluster locally")
  2026-01-21  (log: "paired with Sam on invalidation logic")
  2026-01-22  (note added: cache-invalidation-learnings.md)
              (log: "p99 dropped from 800ms to 120ms")
  2026-01-28  (artifact: linked PR #42)
  2026-02-09  (manual)
```

Each date is a day you can look up in your Obsidian daily note (or wherever your journal lives). Orbit tells you *what kind of activity* happened, but the prose lives in your notes.

### Tag conventions

Tags are free-form, but orbit recognizes two prefixes with special meaning:

| Prefix | Meaning | Cardinality | Example |
|--------|---------|-------------|----------|
| `project:*` | Which project this belongs to | Multiple allowed | `project:payments`, `project:orbit` |
| `owner:*` | Context that owns this work | Single | `owner:work`, `owner:personal` |

In the database, these are just tags — no special treatment. The application layer provides ergonomic commands for managing them (see CLI Design).

This keeps the model flat and flexible: a WorkEntry can belong to multiple projects, projects can span repos, and there's no extra CRUD to manage.

Relationships:
```
WorkEntry 1──* Artifact
WorkEntry 1──* Note       (each Note has a date + file path)
WorkEntry 1──* LogEntry   (timestamped one-liner, stored in DB)
WorkEntry 1──* WorkDay    (date stamp, auto or manual)
WorkEntry *──* Tag        (including project:* tags)
```

Examples of what a WorkEntry graph looks like:
```
WorkEntry: "Add caching to payment flow"
  ├─ status: in-progress
  ├─ description: "Introduce Redis caching layer for the payment
  │    lookup path to reduce p99 latency. Spans payments-service
  │    and the shared client library."
  ├─ scratchpad: C:/Users/me/code/payments-service/.dev/caching-experiments
  ├─ tags: [owner:work, project:payments, caching, perf]
  ├─ artifacts:
  │    ├─ branch: payments-repo/feature/add-cache
  │    ├─ branch: shared-lib/cache-improvements
  │    ├─ PR: https://github.com/org/payments/pull/42
  │    └─ repo: C:/Users/me/code/payments-service
  └─ notes:
       ├─ 2026-01-15 C:/Users/me/notes/payments/caching-approach.md
       └─ 2026-01-22 C:/Users/me/notes/payments/cache-invalidation-learnings.md
  └─ log:
       ├─ 2026-01-20 14:32 — "set up redis cluster locally, hit config issue"
       ├─ 2026-01-21 09:15 — "paired with Sam on invalidation logic"
       └─ 2026-01-22 16:40 — "p99 dropped from 800ms to 120ms after adding cache"
  └─ work days:
       ├─ 2026-01-20 (auto: log)
       ├─ 2026-01-21 (auto: log)
       ├─ 2026-01-22 (auto: note + log)
       └─ 2026-02-09 (manual)
```

## 5. Data Storage

**Approach: a single global local database. Orbit tracks paths; it does not own files.**

```
~/.orbit/
  orbit.db          # single source of truth for all structured data
  config.yaml       # user preferences (default editor, etc.)
```

- `orbit.db` stores WorkEntries, Artifacts, Notes (as path references), Tags, and all relationships.
- **Notes live wherever the user puts them.** When you start a new piece of work, you tell orbit where the note (or note folder) is. Orbit records the path and date — nothing more. Your Obsidian vault, your project folder, a random desktop file — orbit doesn't care.
- **Repos are artifacts.** A WorkEntry can link to one or more repo paths. Multiple WorkEntries can reference the same repo.
- Orbit never moves, copies, or creates files outside `~/.orbit/` unless explicitly asked (e.g., a future `orbit work note --create` convenience command).

Advantages:
- Zero infrastructure — no server, no Docker, no cloud
- A local relational database is fast, reliable, and portable
- Notes stay exactly where you already put them
- No per-repo `.orbit/` clutter — your repos stay clean
- One DB = easy to query across all work entries, all projects

Trade-off: if a note file is moved or deleted outside orbit, the reference goes stale. Orbit handles this in two ways:
1. **`orbit doctor`** — explicit check, reports and optionally cleans up stale references.
2. **Lazy daily check** — on any orbit command, if the last health check was >24h ago, orbit runs a quick background sweep and warns about any stale references. The timestamp of the last check is stored in the DB. This keeps things honest without requiring the user to remember to run `orbit doctor`.

Future consideration: an export format (JSON/YAML) for portability or backup.

### WorkEntry export

Since a WorkEntry is just a bundle of references, it can be exported as a dated YAML snapshot on demand:

```
orbit work export <id>              # Print YAML to stdout
orbit work export <id> -o path.yaml # Write to file
```

Example output:
```yaml
# orbit work export: "Add caching to payment flow"
# exported: 2026-02-09T14:32:00
id: w-3a7f
title: "Add caching to payment flow"
description: |
  Introduce Redis caching layer for the payment lookup path
  to reduce p99 latency. Spans payments-service and the shared
  client library.
status: active
created: 2026-01-14
scratchpad: C:/Users/me/code/payments-service/.dev/caching-experiments
tags:
  - owner:work
  - project:payments
  - caching
  - perf
artifacts:
  - type: branch
    value: payments-repo/feature/add-cache
  - type: branch
    value: shared-lib/cache-improvements
  - type: pr
    value: https://github.com/org/payments/pull/42
  - type: repo
    value: C:/Users/me/code/payments-service
notes:
  - date: 2026-01-15
    path: C:/Users/me/notes/payments/caching-approach.md
  - date: 2026-01-22
    path: C:/Users/me/notes/payments/cache-invalidation-learnings.md
log:
  - timestamp: 2026-01-20T14:32:00
    message: "set up redis cluster locally, hit config issue"
  - timestamp: 2026-01-21T09:15:00
    message: "paired with Sam on invalidation logic"
  - timestamp: 2026-01-22T16:40:00
    message: "p99 dropped from 800ms to 120ms after adding cache"
work_days:
  - date: 2026-01-20
    source: auto (log)
  - date: 2026-01-21
    source: auto (log)
  - date: 2026-01-22
    source: auto (note + log)
  - date: 2026-02-09
    source: manual
```

The export is a point-in-time snapshot (always dated). Future possibility: version-controlled snapshots, e.g., orbit auto-commits exports to a git repo for a full history of how a work entry evolved.

## 6. CLI Design

### Command hierarchy (draft)

```
orbit init                                  # Initialize orbit (create ~/.orbit/)

orbit work new <title>                      # Create a new work entry (status: new)
orbit work list                             # List work entries (filterable)
orbit work list --project payments          # Filter by project
orbit work list --owner work                # Filter by owner
orbit work list --tag caching               # Filter by any tag
orbit work show <id>                        # Show a work entry and all linked artifacts/notes
orbit work show                             # Show selected work entry (if any)
orbit work close <id>                       # Complete a work entry (status: completed)
orbit work close <id> --abandon --reason .. # Abandon with reason
orbit work status <id> <status>             # Set status explicitly (--reason optional)
orbit work tag <id> <tag>                   # Add a tag (e.g., caching, perf)
orbit work tag <id> <tag> --remove          # Remove a tag

orbit work project add <name>               # Add project:* tag (multiple allowed)
orbit work project remove <name>            # Remove project:* tag
orbit work project list                     # List projects for selected/given entry

orbit work owner <name>                     # Set owner:* tag (replaces any existing)
orbit work owner --clear                    # Remove owner tag

orbit work search <query>                   # Full-text search across work entries and notes
orbit work select <id>                      # Set as the selected (current) work entry
orbit work forget                           # Clear the selected work entry
orbit work export <id>                      # Export a work entry as a dated YAML snapshot

orbit work log <message>                    # Append a log entry to the selected work entry
orbit work log <id> <message>               # Append a log entry to a specific work entry
orbit work log list                         # Show log entries for the selected work entry
orbit work log list <id>                    # Show log entries for a specific work entry
orbit work log list --since 1w              # Filter log entries by date
orbit work log list --all                   # Show log entries across all work entries

orbit work today                             # Mark that you worked on the selected entry today
orbit work today <id>                        # Mark for a specific entry
orbit work diary                             # Show work days for the selected entry
orbit work diary <id>                        # Show work days for a specific entry
orbit work diary --since 2w                  # Filter by date range

orbit work scratchpad <path>                 # Set scratchpad folder for selected entry
orbit work scratchpad <id> <path>            # Set scratchpad for a specific entry
orbit work scratchpad --clear                # Remove scratchpad from selected entry
orbit work scratchpad --open                 # Open scratchpad folder in file explorer

orbit link <id> --branch <name>             # Link a git branch
orbit link <id> --pr <url>                  # Link a pull request
orbit link <id> --workitem <url>            # Link an issue or work item (ADO, GitHub)
orbit link <id> --repo <path>               # Link a local repo
orbit link <id> --file <path>               # Link any file or folder
orbit link <id> --url <url>                 # Link any other URL
orbit link <id> --custom <value>            # Link freeform reference
orbit link <id> --note <path> [--date ...]  # Link an existing note (md file or folder)
orbit link --branch <name>                  # Link to selected work entry (id optional)

orbit summary --since 2w                    # Generate a summary of recent work
orbit summary --project payments            # Summary scoped to a project
orbit summary --owner work                  # Summary scoped to an owner
orbit summary --tag caching                 # Summary filtered by any tag

orbit status                                # Quick overview: active work entries, selected entry, recent notes
orbit doctor                                # Check for stale references (moved/deleted files)
```

Design principles:
- **Noun-first** command pattern: `orbit <noun> <verb>` (like `git`, `docker`)
- `orbit work` is the primary command group — `work` is short for `WorkEntry`
- `orbit link` is a top-level command (not `orbit work link`) since linking is a frequent action
- Projects and owners are tags with conventions (`project:*`, `owner:*`) — ergonomic commands provided
- Interactive prompts where helpful, but everything scriptable with flags
- Output is human-readable by default, `--json` flag for machine consumption
- When run inside a git repo, `orbit link` can auto-detect the current branch as a convenience (nice-to-have, not critical)

## 7. Integrations

**Philosophy:** Orbit works *with* what the developer already has. It does not replace any existing tool, and it does not write back to external platforms. External integrations are **read-only context fetching** — when you link an ADO work item or a GitHub PR, orbit may fetch its title, status, and description so you have context locally, but it never modifies the source.

### Local (core)
- **Git:** detect current branch, list recent branches, read commit log
- **File system:** link local folders and files as artifacts

### External context (later, low priority)
- **GitHub / Azure DevOps:** given a linked URL, fetch metadata (title, status, description) for display. Read-only. No auth required for public repos; PAT-based for private.
- These are **not** a priority for early milestones. The core value is in local tracking and notes.

### MCP Server
- Expose orbit's data as an MCP server
- Tools: `search_work_entries`, `get_work_entry`, `get_notes`, `summarize_project`
- Resources: work entries, notes, projects
- Enables: *"What did I learn about X during project Y?"*

## 8. Architecture

```
┌──────────────┐     ┌──────────────┐
│     CLI      │     │  MCP Server  │
└──────┬───────┘     └──────┬───────┘
       │                    │
       └────────┬───────────┘
                │
        ┌───────▼─────────┐
        │   Core Library  │
        ├─────────────────┤
        │  WorkEntry CRUD │
        │  Artifact links │
        │  Note refs      │
        │  Tag system     │
        │  Search/Query   │
        │  Summary gen    │
        └───────┬─────────┘
                │
     ┌──────────┼──────────┐
     │          │          │
┌────▼───┐ ┌────▼────┐ ┌───▼────┐
│ Local  │ │  Git    │ │ GitHub │
│   DB   │ │ Client  │ │ / ADO  │
└────────┘ └─────────┘ └────────┘
```

- **Core Library** — domain models, storage, query logic. No CLI dependency.
- **CLI** — thin CLI layer that calls into core.
- **TUI** — terminal UI for browsing and interacting with orbit data.
- **MCP Server** — MCP server that calls into core (added later).
- **Integrations** — adapters for Git, GitHub, ADO.

## 9. Technical Considerations

This section outlines the key technical requirements without prescribing specific implementations.

| Concern | Requirement |
|---|---|
| **CLI** | Subcommand-based interface with auto-generated help. Must support both interactive prompts and scriptable flags. |
| **Database** | Local relational database. Must support foreign keys, cascade deletes, full-text search. No external server. |
| **MCP Server** | Implements the Model Context Protocol for LLM tool integration. |
| **Git Integration** | Read-only access to branch names, commit logs, and repository metadata. |
| **TUI** | Terminal-based interface for browsing and editing. Keyboard-driven navigation. |
| **External APIs** | HTTP client for fetching metadata from GitHub/ADO. Read-only. PAT-based auth for private repos. |

## 10. Milestones


### M0 — First light
> *"I can create, list, view, and delete work entries from the terminal."*

- [x] Design doc finalized
- [ ] Project scaffold with core library and CLI entrypoint
- [ ] Database schema (WorkEntry table, Tag table, join table)
- [ ] `orbit init` — create `~/.orbit/` and `orbit.db`
- [ ] `orbit work new <title>` — create a work entry (with optional `--description`, `--tag`)
- [ ] `orbit work list` — list all work entries (table output: id, title, status, tags, created)
- [ ] `orbit work show <id>` — show a single work entry
- [ ] `orbit work delete <id>` — delete a work entry (with confirmation prompt)
- [ ] `orbit work tag <id> <tag>` — add/remove tags
- [ ] Unit tests for core CRUD

### M1 — Daily driver
> *"I can track my work through its lifecycle, link artifacts, and take quick notes."*

- [ ] `orbit work status <id> <status>` (with `--reason`), `orbit work close`
- [ ] `orbit work select <id>` / `orbit work forget` — set/clear the selected entry
- [ ] `orbit work show` (no args) — show the selected entry
- [ ] `orbit link` — link artifacts (note, branch, repo, file, URL) to a work entry
- [ ] `orbit link` defaults to selected entry when `<id>` is omitted
- [ ] `orbit work scratchpad` — set/clear/open scratchpad folder
- [ ] `orbit work log <message>` / `orbit work log list`
- [ ] `orbit work today` / `orbit work diary`
- [ ] Auto-recording of work days (on link, log, note actions)
- [ ] `orbit work project add/remove/list` — manage project tags
- [ ] `orbit work owner` — set/clear owner tag
- [ ] `orbit status` — quick overview (selected entry, active entries, recent activity)

### M2 — Find & filter
> *"I can search across everything and keep my data healthy."*

- [ ] `orbit work list` with filtering: `--project`, `--owner`, `--tag`, `--status`, `--since`, `--until`
- [ ] `orbit work search <query>` — full-text search across work entries, log entries, and note content
- [ ] `orbit doctor` — detect stale references (moved/deleted files)
- [ ] Lazy daily health check (auto-runs on any command if >24h since last check)
- [ ] `--json` output flag on list/show/search commands

### M3 — Export, summarize & browse
> *"I can generate reports, export snapshots, and browse my work in a TUI."*

- [ ] `orbit work export <id>` — dated YAML snapshot
- [ ] `orbit summary --since 2w` — summarize recent work
- [ ] `orbit summary --project payments` / `--owner work` — scoped summaries
- [ ] `orbit tui` — read-only TUI browser
  - [ ] Work entry list view (navigate, filter by status/tag)
  - [ ] Work entry detail view (description, artifacts, logs, diary)
  - [ ] Log entry timeline view
  - [ ] Keyboard shortcuts for navigation (j/k, enter, esc, q)

### M4 — Interactive TUI
> *"I can manage my work entirely from the TUI without dropping to the CLI."*

- [ ] Quick actions from TUI:
  - [ ] Add log entry (inline input)
  - [ ] Change status (dropdown/menu)
  - [ ] Select/forget work entry
  - [ ] Mark "worked today"
- [ ] Create new work entry from TUI
- [ ] Link artifacts from TUI (file picker or paste URL)
- [ ] Command palette (`Ctrl+P` or `/`) for quick actions

### M5 — MCP Server
> *"An LLM can answer questions about my work history."*

- [ ] MCP server exposing orbit data
- [ ] Tools: `search_work_entries`, `get_work_entry`, `get_notes`, `summarize_project`
- [ ] Resources: work entries, log entries, work days, notes
