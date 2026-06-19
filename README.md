# orbit

**Your developer universe, mapped and in motion**

---

## What is Orbit?

Orbit is a personal developer work tracker that brings together all the pieces of your workflow — issues, pull requests, local projects, notes, and more — into a single, unified space. It's designed for engineers who want to organize, reflect on, and report their work across platforms, with a simple command-line interface.

Written in Go. Local-first. CLI-first. LLM-ready.

## Features

- **Unified Tracking:** Link issues, pull requests, local folders, notes, and more to a single “work item.”
- **Easy Organization:** Automatically set up folders, notes, and workspace settings for new work.
- **Powerful Querying:** Find your work by date, project, tags, or impact.
- **Reflection & Reporting:** Summarize your progress and lessons learned for reviews or retrospectives.
- **CLI-first:** Fast, scriptable, and designed for developer workflows.

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

### Usage

```sh
orbit init                  # initialize orbit (~/.orbit/)
orbit work new "my task"    # create a work entry
orbit work list             # list work entries
orbit status                # quick overview
```

*Orbit is under active development — see [docs/DESIGN.md](docs/DESIGN.md) for the full vision and milestone plan.*

## Walkthrough: starting a new piece of work

Orbit doesn't impose a folder layout — it indexes references you provide. A typical "starting a new piece of work" flow looks like this:

```sh
# 1. Create the work entry and a scratchpad folder for it
orbit work new "Add caching to payment flow" -s payments-caching
# If `scratchpad.root` is set in ~/.orbit/config.yaml, this creates
#   {scratchpad.root}/payments-caching/
# Otherwise it creates ./payments-caching/ in the current directory.
# Use --no-root to force CWD even when scratchpad.root is set.
# The new entry is auto-selected (use --no-select to opt out).

# 2. Link the things this work touches
orbit link --repo   C:/Users/me/code/payments-service
orbit link --dir    C:/Users/me/docs/payments-design
orbit link --file   C:/Users/me/code/payments.code-workspace
orbit link --branch feature/add-cache
orbit link --note   C:/Users/me/notes/payments/caching-approach.md

# 3. Organize and tag
orbit work project add payments
orbit work owner work
orbit work tag w-3a7f caching
orbit work tag w-3a7f perf

# 4. Work as usual; jot quick observations to the timeline
orbit work log "set up redis cluster locally, hit config issue"
orbit work today                                    # mark today as a work day
```

Later, recall everything in one shot:

```sh
orbit work show w-3a7f      # description, status, tags, artifacts, recent logs
orbit work diary w-3a7f     # timeline of days you touched it, annotated
orbit work export w-3a7f    # dated YAML snapshot of the whole graph
```

The core idea: **orbit holds the index, your repos / notes / scratchpad hold the substance.** Open the linked `.code-workspace` from VS Code (or run `orbit work scratchpad --open`) and resume work — orbit just knows where everything is.

> **Coming soon:** a single `orbit work new <title> -s <name> --repo ... --workspace ... --note ...` to collapse steps 1–3 into one command, and `orbit work open [id]` to open the scratchpad and linked workspace from one command. Tracked in [docs/DESIGN.md](docs/DESIGN.md).

## Documentation

- [Design Document](docs/DESIGN.md) — vision, CLI design, milestones
- [Data Model](docs/DATA_MODEL.md) — entity relationships and schema
- [Tech Stack](docs/TECH_STACK.md) — Go libraries and project layout
