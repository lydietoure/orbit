# Orbit — Tech Stack

> **Status:** Active
> **Last updated:** 2026-06-18

This document describes the concrete technology choices for Orbit's Go implementation. For the product vision, domain model, and CLI design, see [DESIGN.md](DESIGN.md). For the data model, see [DATA_MODEL.md](DATA_MODEL.md).

---

## Language

**Go** — chosen for fast startup, single-binary distribution, and a natural fit for CLI tooling.

---

## Dependencies

| Concern | Library | Why |
|---|---|---|
| **CLI** | [`cobra`](https://github.com/spf13/cobra) | Industry standard. Subcommand trees, auto-generated help, shell completions. Used by `kubectl`, `docker`, `gh`. |
| **Database** | [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) | Pure Go SQLite. No CGo, cross-compiles trivially. |
| **Config (YAML)** | [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) | Mature YAML parser. Reads/writes `config.yaml`. |
| **TUI** | [`bubbletea`](https://github.com/charmbracelet/bubbletea) | Elm-architecture TUI from Charm. Composable, testable. For M3/M4 milestones. |
| **TUI styling** | [`lipgloss`](https://github.com/charmbracelet/lipgloss) | Terminal styling and layout, pairs with bubbletea. |
| **Git** | `os/exec` → `git` CLI | Shell out to `git` for branch detection, commit logs. No library needed. |
| **MCP Server** | `net/http` + `encoding/json` (stdlib) | Lightweight JSON-RPC server. No framework needed for M5. |

---

## Project Layout

```
orbit/
├── cmd/
│   └── orbit/
│       └── main.go              # Entrypoint
├── internal/
│   ├── cli/                     # Cobra command definitions
│   │   ├── root.go              # Root command, version flag
│   │   ├── work.go              # orbit work *
│   │   ├── link.go              # orbit link *
│   │   ├── log.go               # orbit log *  (if promoted to top-level)
│   │   └── ...
│   ├── core/                    # Domain logic (no CLI or DB imports)
│   │   ├── work_entry.go        # WorkEntry type + business rules
│   │   ├── tag.go
│   │   ├── artifact.go
│   │   └── ...
│   ├── db/                      # SQLite access layer
│   │   ├── db.go                # Open, migrate, pragma setup
│   │   ├── schema.sql           # Embedded via go:embed
│   │   ├── work_entry_repo.go   # WorkEntry queries
│   │   └── ...
│   ├── config/                  # YAML config load/save
│   │   └── config.go
│   └── tui/                     # Bubbletea TUI (M3+)
│       └── ...
├── docs/
│   ├── DESIGN.md
│   ├── DATA_MODEL.md
│   └── TECH_STACK.md
├── go.mod
├── go.sum
└── README.md
```

### Key conventions

- **`cmd/`** — only `main.go`, wires everything together.
- **`internal/`** — all packages are internal (not importable by external modules). This is a personal tool, not a library.
- **`internal/core/`** — pure domain types and logic. No database, no CLI, no I/O imports. Testable in isolation.
- **`internal/db/`** — repository pattern. Each entity gets a `*_repo.go` file with CRUD functions. Schema is embedded with `//go:embed schema.sql`.
- **`internal/cli/`** — thin layer. Each cobra command calls into `core/` or `db/`. No business logic here.

---

## Database

- **Engine:** SQLite via `modernc.org/sqlite`
- **Location:** `~/.orbit/orbit.db` 
- **Schema:** See [`DATA_MODEL.md`](DATA_MODEL.md) and `internal/db/schema.sql`
- **Pragmas:** `foreign_keys = ON`, `journal_mode = WAL`
- **Schema embedding:** `//go:embed schema.sql` in `internal/db/db.go`

## Build & Distribution

- **Build:** `go build -o orbit ./cmd/orbit`
- **Output:** Single static binary, no runtime dependencies
- **Cross-compilation:** `GOOS=windows GOARCH=amd64 go build ...` etc.
- **Task runner:** [`Taskfile.yml`](https://taskfile.dev/) (carried over from the Python era)

---

## Testing

- **Framework:** `testing` (stdlib) + `testify` for assertions if needed
- **Database tests:** Use in-memory SQLite (`:memory:`) for fast, isolated tests
- **Strategy:** Unit tests for `core/`, integration tests for `db/`, minimal tests for `cli/`
