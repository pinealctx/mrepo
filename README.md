# mrepo

A CLI and TUI tool for managing multiple Git repositories within a monorepo workspace.

## Features

- **Parallel status** — query all repos concurrently, see branch, worktree status, ahead/behind
- **Parallel pull/fetch** — `git pull` or `git fetch` all repos with bounded concurrency
- **Interactive TUI** — terminal dashboard with keyboard navigation, built with Bubble Tea
- **Config-driven** — `.repos.toml` or `.repos.yaml` tracks your repos and groups
- **Auto-discovery** — `scan` finds untracked Git repos in your workspace
- **JSON output** — `--json` flag on all commands for scripting integration
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

# Add all found repos
mrepo scan --add

# Check status of all repos
mrepo status

# Pull latest for all repos
mrepo pull

# Launch interactive TUI
mrepo tui
```

## Configuration

mrepo looks for `.repos.toml`, `.repos.yml`, or `.repos.yaml` in the root directory.

### TOML Example (`.repos.toml`)

```toml
version = 1

[repos.backend]
path = "services/backend"
description = "Go API server"

[repos.frontend]
path = "web/frontend"
description = "React web app"

[groups.services]
repos = ["backend"]
```

### YAML Example (`.repos.yaml`)

```yaml
version: 1
repos:
  backend:
    path: services/backend
    description: Go API server
  frontend:
    path: web/frontend
    description: React web app
groups:
  services:
    repos:
      - backend
```

## Commands

| Command | Description |
|---------|-------------|
| `mrepo status` | Show status of all repos (branch, clean/dirty, ahead/behind) |
| `mrepo pull` | Pull latest changes for all repos in parallel |
| `mrepo fetch` | Fetch latest refs for all repos in parallel |
| `mrepo add <path>` | Register a new repo |
| `mrepo remove <name>` | Remove a repo from config (`--delete --force` to delete directory) |
| `mrepo scan` | Discover untracked Git repos (`--add` to register them) |
| `mrepo tui` | Launch interactive terminal dashboard |

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--root` | `.` | Root directory of the monorepo |
| `--format` | auto | Config file format (`toml`, `yaml`) |
| `--json` | `false` | Output as JSON (status, scan) |

## TUI Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `s` | Refresh status |
| `p` | Pull all repos |
| `f` | Fetch all repos |
| `Enter` | View repo detail (recent commits) |
| `Esc` | Back to list |
| `q` | Quit |

## License

[MIT](LICENSE)
