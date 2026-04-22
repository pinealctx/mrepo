# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

mrepo is a Go CLI and TUI tool for managing multiple Git repositories within a monorepo workspace. It supports cloning repos from config, parallel status/pull/fetch operations, running arbitrary commands across repos (`forall`), group-based filtering, and an interactive Bubble Tea terminal dashboard.

Module path: `github.com/pinealctx/mrepo`

## Commands

```bash
go build .                                    # Build binary
go test -failfast ./...                       # Run tests
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
  cmd/                         Cobra commands: root, status, pull, fetch, clone, sync,
                               forall, add, remove, scan, group, tui, version
  config/                      Config loading/saving (TOML + YAML), repo registry,
                               validation on Load (path/remote format checks)
  git/                         Git CLI wrappers (status, clone, pull, fetch, scan,
                               repo info, log), parallelDo generic helper
  tui/                         Bubble Tea v2 TUI: left (repos│branches│files) right (diff)
  version/                     Version string, injected via -ldflags at release
```

### Config structure

Each repo entry has `path`, `remote` (clone URL, optional), `branch` (optional), and `description` (optional). Config is validated on load: paths must not be empty or start with `-`; remotes must look like URLs or SSH addresses.

```toml
version = 1

[repos.backend]
path = "services/backend"
remote = "https://github.com/org/backend.git"
branch = "main"
```

Groups can be defined to filter repos via `--group` flag:

```toml
[groups.services]
repos = ["backend", "frontend"]
```

### Data flow

1. `cmd/` loads config via `loadConfig(rootDir)` (auto-detects `.repos.toml` / `.repos.yml` / `.repos.yaml`, validates entries)
2. `filterRepos(cfg)` applies `--group` filtering if set, returns `map[string]*config.Repo`
3. `partitionRepos()` splits into existing/missing sets (used by pull, fetch)
4. Builds a `map[string]string` of `{repoName: relativePath}` for git operations
5. Calls `git.GetStatuses()` / `git.PullAll()` / `git.CloneAll()` which fan out via `parallelDo` generic helper using `errgroup` with `runtime.NumCPU()` workers and `atomic.Int64` index
6. Each worker shells out to `git` via `exec.CommandContext`
7. Results sorted alphabetically and rendered via `newResultTable()` / `newHeaderTable()` or `printJSON()`

### Key CLI commands

| Command | Description |
|---------|-------------|
| `mrepo status` | Show status (branch, clean/dirty/missing, ahead/behind) |
| `mrepo status --branches` | Show all local branches per repo with ahead/behind |
| `mrepo clone` | Clone repos not yet on disk (`--force`, `--depth N`) |
| `mrepo sync` | Clone missing + pull existing in one step |
| `mrepo pull` | Pull existing repos (skips missing) |
| `mrepo fetch` | Fetch refs for existing repos |
| `mrepo checkout <branch>` | Checkout a branch across repos (`--create` to create new) |
| `mrepo forall -- <cmd>` | Run a command in each repo |
| `mrepo scan` | Discover untracked repos (`--add` auto-detects remote/branch) |
| `mrepo group list` | List groups and their repos |
| `mrepo group create/delete/add/remove` | Manage groups |

### Global flags

- `--root` — workspace root (default `.`)
- `--group` — filter by group name (all commands including TUI)
- `--json` — JSON output on reporting commands

### Key dependencies

- `github.com/spf13/cobra` — CLI framework
- `charm.land/bubbletea/v2` — TUI framework (Bubble Tea v2)
- `charm.land/lipgloss/v2` + `charm.land/lipgloss/v2/table` — styling, layout (JoinHorizontal), static table renderer (CLI output)
- `golang.org/x/sync/errgroup` — bounded parallelism
- `github.com/pelletier/go-toml/v2` + `gopkg.in/yaml.v3` — config formats

### Key patterns

- `parallelDo[T]` generic in `git/operations.go` eliminates parallel worker boilerplate
- `filterRepos()` in `cmd/root.go` centralizes `--group` filtering for all commands (including TUI)
- `loadConfig()` in `cmd/root.go` centralizes config file lookup + loading (used by all commands)
- `printJSON()` / `newResultTable()` / `newHeaderTable()` in `cmd/root.go` eliminate output boilerplate
- `partitionRepos()` in `cmd/root.go` splits repos into existing/missing (used by pull, fetch)
- `OperationResult` in `git/operations.go` is the unified result type for pull and fetch operations
- Timeout constants (`statusTimeout`, `pullTimeout`, etc.) defined in `cmd/root.go`
- `truncate()` in `cmd/pull.go` counts runes (not bytes) for safe UTF-8 truncation
- `validateCloneTarget()` prevents path traversal and flag injection in clone operations
- CLI table output uses `lipgloss/v2/table` (static renderer with `StyleFunc`); TUI uses manual rendering with `lipgloss.JoinHorizontal` for master-detail split layout
- TUI layout: left panel (repos│branches│files) split vertically + right panel (file diff), `tab` cycles focus sections
- TUI focus model: 3 sections (repos/branches/files), `tab` cycles, `j/k` moves within, `enter` acts (checkout branch / load diff)
- `padRight(s, width)` in `cmd/forall.go` pads plain-text strings — only used by `forall`
- `ensureGitignore()` / `removeFromGitignore()` auto-manage `.gitignore` entries for sub-repos

### Conventions

- Conventional Commits: `<type>(<scope>): <description>` — imperative mood, lowercase, no period, max 72 chars
- No backward compatibility — project is in initial development
- Go style: Effective Go + Uber Go Style Guide. `PascalCase`/`camelCase`, acronyms as `UserID`
- All reporting commands support `--json`; parallel operations use `errgroup` + `atomic.Int64`
- `errcheck` linter is enabled — all error returns must be checked

## Pre-commit Hooks

`.pre-commit-config.yaml` runs: gitleaks, trailing-whitespace/EOF fixes, `go mod tidy`, `go build`, `go test`, `golangci-lint`. Push hook blocks direct commits to main/master.
