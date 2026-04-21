# mrepo

A CLI and TUI tool for managing multiple Git repositories within a monorepo workspace.

## Features

- **Clone & sync** — bootstrap repos from config with `mrepo clone` or `mrepo sync`
- **Parallel status** — query all repos concurrently, see branch, worktree status, ahead/behind, missing
- **Parallel pull/fetch** — `git pull` or `git fetch` all repos with bounded concurrency
- **Run commands** — `mrepo forall -- make build` runs a command in every repo
- **Group filtering** — `--group services` to operate on a subset of repos
- **Shallow clones** — `--depth 1` for fast bootstrap of large repos
- **Interactive TUI** — terminal dashboard with clone/sync/pull/fetch, built with Bubble Tea
- **Config-driven** — `.repos.toml` or `.repos.yaml` tracks repos, remotes, branches, and groups
- **Auto-discovery** — `scan` finds untracked Git repos and auto-detects remote URLs
- **JSON output** — `--json` flag on all reporting commands for scripting integration
- **Cross-platform** — single Go binary, works on Linux, macOS, Windows

## Install

```bash
go install github.com/pinealctx/mrepo@latest
```

Or download a binary from [Releases](https://github.com/pinealctx/mrepo/releases).

## Quick Start

```bash
# Scan for Git repos in current directory
mrepo scan

# Add all found repos (auto-detects remote URL and branch)
mrepo scan --add

# Check status of all repos
mrepo status

# Pull latest for all repos
mrepo pull

# Clone missing repos + pull existing in one step
mrepo sync

# Run a command in every repo
mrepo forall -- go test ./...

# Launch interactive TUI
mrepo tui
```

## Configuration

mrepo looks for `.repos.toml`, `.repos.yml`, or `.repos.yaml` in the root directory.

Each repo can optionally specify a `remote` (clone URL) and `branch` to enable `clone` and `sync`.

### TOML Example (`.repos.toml`)

```toml
version = 1

[repos.backend]
path = "services/backend"
remote = "https://github.com/org/backend.git"
branch = "main"
description = "Go API server"

[repos.frontend]
path = "web/frontend"
remote = "git@github.com:org/frontend.git"
branch = "main"
description = "React web app"

[groups.services]
repos = ["backend", "frontend"]
```

### YAML Example (`.repos.yaml`)

```yaml
version: 1
repos:
  backend:
    path: services/backend
    remote: https://github.com/org/backend.git
    branch: main
    description: Go API server
  frontend:
    path: web/frontend
    remote: "git@github.com:org/frontend.git"
    branch: main
    description: React web app
groups:
  services:
    repos:
      - backend
      - frontend
```

## Commands

| Command | Description |
|---------|-------------|
| `mrepo status` | Show status of all repos (branch, clean/dirty/missing, ahead/behind) |
| `mrepo clone` | Clone repos not yet on disk (`--force` to re-clone, `--depth N` for shallow) |
| `mrepo sync` | Clone missing + pull existing in one step (`--depth N` for shallow clones) |
| `mrepo pull` | Pull latest changes for all repos in parallel (skips missing) |
| `mrepo fetch` | Fetch latest refs for all repos in parallel |
| `mrepo forall -- <cmd>` | Run a command in each repo (e.g., `mrepo forall -- make build`) |
| `mrepo add <path>` | Register a new repo (auto-detects remote/branch) |
| `mrepo remove <name>` | Remove a repo from config (`--delete --force` to delete directory) |
| `mrepo scan` | Discover untracked Git repos (`--add` to register with remote/branch) |
| `mrepo version` | Print the version |
| `mrepo tui` | Launch interactive terminal dashboard |

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--root` | `.` | Root directory of the monorepo |
| `--format` | auto | Config file format (`toml`, `yaml`) |
| `--group` | | Filter repos by group name |
| `--json` | `false` | Output as JSON (on reporting commands) |

## TUI Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `s` | Refresh status |
| `p` | Pull all repos |
| `f` | Fetch all repos |
| `c` | Clone missing repos |
| `S` (shift) | Sync all (clone + pull) |
| `Enter` | View repo detail (recent commits) |
| `Esc` | Back to list |
| `q` | Quit |

## License

[MIT](LICENSE)
