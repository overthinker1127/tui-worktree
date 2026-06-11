# Worktree Diff TUI

## Goal

- Implement the first vertical slice of a Go TUI that displays current git worktree changes in a GitHub PR-style files changed view, with reusable theme support.

## References

- `docs/AGENTS.md`
- `docs/STATE.md`
- `docs/ROADMAP.md`
- `docs/ARCHITECTURE.md`
- `docs/v1/designs/2026-06-11-v1-worktree-diff-tui.md`

## Workspace

- Branch: feat/v1-worktree-diff-tui
- Base: main
- Isolation: required
- Created by: `planning worktree-diff-tui`

## Task Graph

### Task T1

- Goal: Initialize the Go CLI module and dependency baseline.
- Depends on:
  - none
- Write Scope:
  - `go.mod`
  - `go.sum`
  - `cmd/worktree-diff-tui/...`
  - `internal/app/...`
- Read Context:
  - `docs/v1/designs/2026-06-11-v1-worktree-diff-tui.md`
- Checks:
  - `go mod tidy`
  - `go test ./...`
- Parallel-safe: no

### Task T2

- Goal: Implement git command adapter and parsers for changed files, numstat, and per-file diff output.
- Depends on:
  - T1
- Write Scope:
  - `internal/git/...`
  - `internal/git/testdata/...`
- Read Context:
  - `docs/ARCHITECTURE.md`
  - `docs/v1/designs/2026-06-11-v1-worktree-diff-tui.md`
- Checks:
  - `go test ./internal/git/...`
- Parallel-safe: yes

### Task T3

- Goal: Implement theme tokens, dark/light presets, and Lip Gloss style construction.
- Depends on:
  - T1
- Write Scope:
  - `internal/theme/...`
- Read Context:
  - `docs/v1/designs/2026-06-11-v1-worktree-diff-tui.md`
- Checks:
  - `go test ./internal/theme/...`
- Parallel-safe: yes

### Task T4

- Goal: Implement the Bubble Tea model, keybindings, list navigation, diff viewport, and error states.
- Depends on:
  - T2
  - T3
- Write Scope:
  - `internal/tui/...`
- Read Context:
  - `internal/git/...`
  - `internal/theme/...`
  - `docs/v1/designs/2026-06-11-v1-worktree-diff-tui.md`
- Checks:
  - `go test ./internal/tui/...`
  - `go test ./...`
- Parallel-safe: no

### Task T5

- Goal: Wire the CLI entrypoint, repository path detection, theme selection flag, and terminal-safe app startup.
- Depends on:
  - T4
- Write Scope:
  - `cmd/worktree-diff-tui/...`
  - `internal/app/...`
- Read Context:
  - `internal/tui/...`
  - `internal/git/...`
  - `internal/theme/...`
- Checks:
  - `go test ./...`
  - `go run ./cmd/worktree-diff-tui --help`
- Parallel-safe: no

### Task T6

- Goal: Validate the full read-only viewer against a fixture git repository and document the manual smoke path.
- Depends on:
  - T5
- Write Scope:
  - `README.md`
  - `internal/git/testdata/...`
- Read Context:
  - `docs/v1/designs/2026-06-11-v1-worktree-diff-tui.md`
- Checks:
  - `go test ./...`
  - `manual: create a temporary git repository with modified, added, deleted, renamed, and binary files`
  - `manual: run the TUI and verify file list, add/delete counts, diff scrolling, theme switching, quit, and terminal recovery`
- Parallel-safe: no

## Notes

- Execution uses an isolated git worktree from `main`.
- The design leaves module and binary names open. Use `worktree-diff-tui` unless the user chooses a different name before implementation.
- MVP should shell out to local `git`; defer go-git migration until the read-only viewer proves its shape.
