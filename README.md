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

- [Go 1.24+](https://go.dev/dl/)
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

## Documentation

- [Design Document](docs/DESIGN.md) — vision, CLI design, milestones
- [Data Model](docs/DATA_MODEL.md) — entity relationships and schema
- [Tech Stack](docs/TECH_STACK.md) — Go libraries and project layout
