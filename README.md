# orbit

**Your developer universe, mapped and in motion**

---

## What is Orbit?

Orbit is a personal developer work tracker that brings together all the pieces of your workflow — issues, pull requests, local projects, notes, and more — into a single, unified space. It's designed for engineers who want to organize, reflect on, and report their work across platforms, with a simple command-line interface.

Written in Go. Local-first. CLI-first. LLM-ready.

## What 0.1.0 ships

0.1.0 is the foundation. Linking, querying, and reporting come in
later milestones (see [Roadmap](#roadmap)). Today you can:

- **Track work entries** with title, status, tags, project, and owner.
- **Select one entry as the current focus** so other commands default to it.
- **Attach a "pad" folder** to each entry — auto-provisioned under a
  configurable dock root, or anywhere you choose.
- **Store everything locally** in a SQLite database under `~/.orbit/`.
- **Script against it** with POSIX exit codes (`0`/`1`/`2`), `--yes`
  to skip prompts, and machine-friendly output where it matters.

## Getting Started

### Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Task](https://taskfile.dev/) (optional, for build tasks)

### Build

```sh
task build        # builds to dist/orbit
# or
mkdir -p dist && go build -o dist/orbit ./cmd/orbit
```

For day-to-day development — tests, smoke runs against a throwaway
orbit home, environment variables — see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

### Quick start

```sh
orbit init                                      # ~/.orbit/{config,orbit.db}
orbit config dock set ~/work                    # where pads live (optional)

orbit work new "Add caching to payments" -p payments-caching
# Creates the entry, provisions {dock}/payments-caching/, and selects
# the new entry as the current focus. --no-dock keeps it in CWD;
# --no-select skips auto-selection.

orbit work tag caching                          # tag the selected entry
orbit work show                                 # details of selected
orbit work pad show                             # the pad path + existence

orbit work list                                 # everything
orbit work select <id>                          # change focus
orbit work delete --purge                       # remove entry + pad folder
```

`orbit --help` and `orbit work --help` list everything that ships.

## Walkthrough

Orbit doesn't impose a folder layout — it indexes things you tell it
about. A 0.1.0 flow looks like this:

```sh
# 1. Spin up an entry with a pad folder
orbit work new "Add caching to payments" -p payments-caching
# Auto-selected. Pad lives at {dock-root}/payments-caching/ when a
# dock root is set, otherwise ./payments-caching/.

# 2. Annotate it
orbit work tag caching
orbit work tag perf

# 3. Move the pad later if you change your mind
orbit work pad set ~/scratch/payments
orbit work pad show

# 4. Switch focus when something else interrupts you
orbit work new "Triage flaky test"
orbit work selected                             # the new one
orbit work select <old-id>                      # back to caching

# 5. Wrap up
orbit work delete --purge                       # entry + pad folder, with prompts
```

The core idea: **orbit holds the index; your repos, notes, and pad
folders hold the substance.** 0.1.0 makes that index solid. Linking
external artifacts, logging activity, and reporting come next.

## Roadmap

The vision in [docs/DESIGN.md](docs/DESIGN.md) goes well beyond
0.1.0. The CLI does **not** yet ship:

- `orbit link` — attach repos, dirs, files, branches, and notes to an entry
- `orbit work log` / `today` / `diary` / `export` — reflection & reporting
- `orbit status` — at-a-glance overview of recent activity
- `orbit work pad open` — launch the pad in your editor
- Richer queries by project, owner, date range, and impact

See [docs/DESIGN.md](docs/DESIGN.md) for the full plan.

## Documentation

- [Design Document](docs/DESIGN.md) — vision, CLI design, milestones
- [Data Model](docs/DATA_MODEL.md) — entity relationships and schema
- [Tech Stack](docs/TECH_STACK.md) — Go libraries and project layout
- [Development](docs/DEVELOPMENT.md) — build, test, smoke harness, env vars

## License

MIT — see [LICENSE](LICENSE).
