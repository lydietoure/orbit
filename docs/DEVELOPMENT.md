# Development

This guide covers everything you need to build, test, and iterate on
orbit. End-user docs live in the [README](../README.md); the design and
data model live in [DESIGN.md](DESIGN.md) and
[DATA_MODEL.md](DATA_MODEL.md).

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Task](https://taskfile.dev/) — optional but recommended; every
  command below has a Task wrapper

## Common commands

```sh
task build       # build the binary to dist/orbit
task test        # run all unit tests
task lint        # run go vet
task check       # lint + test
task clean       # remove dist/
```

Equivalent raw `go` commands work too:

```sh
go build -o dist/orbit ./cmd/orbit
go test ./...
go vet ./...
```

## Environment variables

| Variable        | Purpose                                                     |
|-----------------|-------------------------------------------------------------|
| `ORBIT_HOME`    | Override the orbit home directory (default `~/.orbit`)      |
| `ORBIT_DOCK`    | Override the dock root (pad parent dir); beats the persisted `orbit config pad set-root` value |
| `ORBIT_VERBOSE` | Equivalent to `--verbose` / `-v` (truthy: `1`, `true`, …)   |
| `ORBIT_DEBUG`   | Equivalent to `--debug` (implies verbose)                   |

Explicit CLI flags always win over env vars. Truthy values are case-
insensitive: `1`, `true`, `yes`, `on`. Anything else is false.

## Smoke testing against a throwaway home

When iterating on commands that write to the orbit home (`init`, soon
`work new`, etc.) you don't want to pollute your real `~/.orbit`. The
Taskfile provides a smoke harness for that.

### .env

The repo loads a gitignored `.env` file at the workspace root via
`dotenv:` in [Taskfile.yml](../Taskfile.yml). Set local-only knobs
there. Example:

```env
# Where `task smoke` points orbit. No default — must be set
# explicitly here or on the CLI.
ORBIT_HOME=.local/smoke

# Verbose logging on by default in smoke runs. Comment out for silence.
ORBIT_VERBOSE=1
```

`ORBIT_HOME` has **no fallback default** in the Taskfile. The smoke
tasks `requires:` it — they fail loudly if it's unset, so an empty
value can never trigger a `rm -rf /*`.

### Tasks

```sh
task smoke -- init                          # run orbit init against $ORBIT_HOME
task smoke -- -v init                       # forward flags to orbit
task smoke ORBIT_HOME=.local/foo -- init    # CLI override beats .env
task smoke:clean                            # delete $ORBIT_HOME entirely
task smoke:reset -- -v                      # delete + re-init
```

Mechanics:

- `task smoke` runs `go run ./cmd/orbit` with everything after `--`
  forwarded as args. `ORBIT_HOME` is exported into the child process
  via the `dotenv:` directive (no separate indirection variable).
  Orbit itself owns its home-dir lifecycle (created by `init`,
  removed by `destroy`), so the harness deliberately does not
  pre-create `$ORBIT_HOME` — that keeps the "not initialized" error
  path honest.
- `task smoke:clean` `rm -rf`s `$ORBIT_HOME`. Safe because the
  Taskfile `requires:` the var, so an empty value is impossible.
- `task smoke:reset` is `smoke:clean` followed by `smoke -- init`,
  forwarding any extra `--` args to the second step.

## Project layout

See [TECH_STACK.md](TECH_STACK.md) for the full layout and library
choices. At a glance:

```
cmd/orbit/        # main()
internal/cli/     # cobra commands
internal/config/  # paths, env, default config template
internal/db/      # schema + open/initialize
internal/diag/    # logging setup (slog + charmbracelet/log)
internal/core/    # domain types
docs/             # design, data model, this file
_examples/        # ad-hoc demos (excluded from ./... by the leading _)
.local/           # local-only scratch (untracked content; safe for smoke)
```
