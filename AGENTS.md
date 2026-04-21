# mrepo Development Guide

## Project Overview

mrepo is a CLI and TUI tool for managing multiple Git repositories within a
monorepo workspace. It provides parallel status checking, pulling, fetching,
and an interactive terminal dashboard built with Bubble Tea.

The module path is `github.com/pinealctx/mrepo`.

## Architecture

```
main.go                              CLI entry point
internal/
  cmd/                               Cobra CLI commands (root, status, pull, fetch, add, remove, scan, tui)
  config/                            TOML/YAML config loading, repo registry management
  git/                               Git command wrappers (status, pull, fetch, scan)
  tui/                               Bubble Tea TUI dashboard with lipgloss styling
```

### Key Dependencies

- **`github.com/spf13/cobra`**: CLI framework
- **`github.com/charmbracelet/bubbletea`**: TUI framework
- **`charm.land/lipgloss/v2`**: Terminal styling
- **`golang.org/x/sync/errgroup`**: Parallel git operations with bounded concurrency
- **`github.com/pelletier/go-toml/v2`**: TOML config support
- **`gopkg.in/yaml.v3`**: YAML config support

### Key Patterns

- **Parallel execution**: All git operations use `errgroup` with
  `runtime.NumCPU()` workers
- **Config file auto-detection**: Looks for `.repos.toml`, `.repos.yml`,
  `.repos.yaml` in order
- **JSON output**: All commands support `--json` for scripting integration
- **Status aggregation**: Single pass collects branch, worktree status,
  and remote tracking info

## Build/Test/Lint Commands

- **Build**: `go build .`
- **Test**: `go test ./...`
- **Lint**: `golangci-lint run --config=.golangci.yml --timeout=5m`
- **Format**: `gofumpt -w .`
- **Run**: `go run . --root /path/to/monorepo status`

## Code Style

- Standard Go conventions (Effective Go + Uber Go Style Guide)
- PascalCase exported, camelCase private
- Acronyms: `UserID`, `APIKey`, `HTTPClient`
- `context.Context` as first parameter for operations
- Error wrapping with `fmt.Errorf`
- No backward compatibility measures — initial development phase

## Committing

- Conventional Commits: `<type>(<scope>): <description>`
- Imperative mood, lowercase, no period, max 72 chars
