# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

mrepo is a Go CLI and TUI tool for managing multiple Git repositories within a monorepo workspace. It can clone missing repos from config, run parallel status/pull/fetch operations, and provides an interactive Bubble Tea terminal dashboard.

Module path: `github.com/pinealctx/mrepo`

## Commands

```bash
go build .                                    # Build binary
go test -race -failfast ./...                 # Run tests
golangci-lint run --config=.golangci.yml --timeout=5m  # Lint
gofumpt -w .                                  # Format
go run . --root /path/to/monorepo status      # Run against a workspace
task test                                     # Via Taskfile
task lint / task lint:fix                     # Via Taskfile
```

Release: push a semver tag (`vX.Y.Z`) — GoReleaser builds cross-platform binaries via CI.

## Architecture

```
main.go                        Entry point → cmd.Execute()
internal/
  cmd/                         Cobra commands: root, status, pull, fetch, clone, sync, add, remove, scan, tui
  config/                      Config loading/saving (TOML + YAML), repo registry with remote/branch
  git/                         Git CLI wrappers (status, clone, pull, fetch, scan, repo info)
  tui/                         Bubble Tea TUI with lipgloss v2 styling
  version/                     Version string, injected via -ldflags at release
```

### Config structure

Each repo entry has `path`, `remote` (clone URL), `branch` (optional), and `description` (optional). `remote` enables `clone` and `sync` to bootstrap repos that don't exist locally.

```toml
version = 1

[repos.backend]
path = "services/backend"
remote = "https://github.com/org/backend.git"
branch = "main"
```

### Data flow

1. `cmd/` loads config via `config.FindConfigFile()` → `config.Load()` (auto-detects `.repos.toml` / `.repos.yml` / `.repos.yaml`)
2. Builds a `map[string]string` of `{repoName: relativePath}` for git operations
3. Calls `git.GetStatuses()` / `git.PullAll()` / `git.CloneAll()` which fan out via `errgroup` with `runtime.NumCPU()` workers
4. Each worker shells out to `git` via `exec.CommandContext`
5. Results sorted alphabetically and rendered (table/JSON/TUI)

### Key CLI commands

| Command | Description |
|---------|-------------|
| `mrepo status` | Show status (branch, clean/dirty/missing, ahead/behind) |
| `mrepo clone` | Clone repos that don't exist locally (`--force` to re-clone) |
| `mrepo sync` | Clone missing + pull existing in one step |
| `mrepo pull` | Pull existing repos (skips missing) |
| `mrepo fetch` | Fetch refs for existing repos |
| `mrepo scan` | Discover untracked repos (`--add` auto-detects remote/branch) |

### Key dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/charmbracelet/bubbletea` + `charm.land/lipgloss/v2` — TUI
- `golang.org/x/sync/errgroup` — bounded parallelism
- `github.com/pelletier/go-toml/v2` + `gopkg.in/yaml.v3` — config formats

### Conventions

- Conventional Commits: `<type>(<scope>): <description>` — imperative mood, lowercase, no period, max 72 chars
- No backward compatibility — project is in initial development
- Go style: Effective Go + Uber Go Style Guide. `PascalCase`/`camelCase`, acronyms as `UserID`
- All commands support `--json` for scripting; `--root` sets workspace root (default `.`)
- Parallel operations use the errgroup + pre-allocated slice + mutex pattern

## Pre-commit Hooks

`.pre-commit-config.yaml` runs: gitleaks, trailing-whitespace/EOF fixes, `go mod tidy`, `go build`, `go test`, `golangci-lint`. Push hook blocks direct commits to main/master.
