---
feature: worktree-diff-tui
status: plan_ready
created_at: 2026-06-11T09:40:29+09:00
---

# Worktree Diff TUI

## Goal

Build a Go-based terminal UI that shows current worktree changes in a lazygit/GitHub PR files-changed style view, with theme support designed in from the first version.

## Context / Inputs

- Source docs:
  - `docs/STATE.md`
  - `docs/ROADMAP.md`
  - `docs/ARCHITECTURE.md`
- Existing system facts:
  - The workspace is currently empty and is not a git repository.
  - No Go module exists yet.
- External constraints:
  - The user wants modern styling and chose Go + Bubble Tea/Bubbles/Lip Gloss.
  - v1 should stay read-only and focused on viewing local changes.

## Problem Statement

Developers need a focused TUI to inspect the current repository's changed files and per-file diffs without opening a browser or full Git client. The first version should make worktree changes scannable, support keyboard navigation, and establish a theme system so future UI work does not require rewriting rendering code.

## Decision Drivers

- Fast MVP delivery in Go.
- Modern terminal styling and theme tokens from the start.
- GitHub PR-style changed-file review flow.
- Clear boundaries between git data collection, UI state, and rendering.
- Read-only behavior in v1 to reduce risk.
- Minimal dependency risk and active TUI ecosystem.

## Options Considered

### Option A: Go + Bubble Tea v2 + Bubbles + Lip Gloss

- Summary: Use Bubble Tea for event/state architecture, Bubbles for list/viewport/help primitives, and Lip Gloss for themeable styling.
- Pros:
  - Modern TUI styling fits the requested direction.
  - List, viewport, keybinding, and help components map directly to the desired UI.
  - Go delivery speed is strong for a local CLI.
  - Charm ecosystem is active and cohesive.
- Cons:
  - Elm-style update loop requires disciplined model boundaries.
  - Complex panes require careful layout arithmetic.
- Risks:
  - Bubble Tea v2 import paths and APIs should be pinned and verified while scaffolding.

### Option B: Rust + Ratatui

- Summary: Use Rust and Ratatui for a high-performance immediate-mode TUI.
- Pros:
  - Strong rendering performance and typed architecture.
  - Good fit for complex terminal dashboards.
- Cons:
  - Slower MVP for this project if the desired implementation language is Go.
  - Theme and component primitives require more assembly than the Charm stack.
- Risks:
  - More initial complexity before proving the product flow.

### Option C: Go + tview

- Summary: Use tview's higher-level widget set for a Go TUI.
- Pros:
  - Mature widgets for lists, trees, tables, and text views.
  - Good for quickly composing form/data UIs.
- Cons:
  - Less aligned with the requested modern styling direction.
  - Theme system would likely be more app-specific.
- Risks:
  - The UI may feel more conventional than the target GitHub/lazygit-inspired experience.

## Recommended Option

- Choice: Option A, Go + Bubble Tea v2 + Bubbles + Lip Gloss.
- Why now:
  - It best matches the modern styling requirement while keeping MVP implementation direct.
  - Bubbles provides the core UI primitives needed for file list, diff viewport, help, and keybindings.
  - Lip Gloss gives a clean theme token layer early.
- Rejected alternatives:
  - Rust + Ratatui is strong but not the fastest path for this Go-oriented library.
  - tview is practical but less ideal for the desired visual system.

## Scope Decision

- In:
  - Initialize a Go CLI module.
  - Build a read-only full-screen TUI.
  - Collect current worktree changes from local `git` commands.
  - Render changed files with status and add/delete counts.
  - Render selected file diff in a scrollable preview.
  - Provide keyboard navigation and quit behavior.
  - Add theme presets and a reusable theme/style boundary.
  - Include modern presets inspired by TokyoNight and Kanagawa.
- Out:
  - Commit, push, pull, branch, and PR creation.
  - Staging/unstaging files or hunks.
  - Conflict resolution.
  - Authentication or network GitHub integration.
  - Full git history/blame views.
- Deferred:
  - Replace shell-based git adapter with go-git if MVP behavior requires library-level control.
  - Add theme config file loading.
  - Add hunk-level navigation and syntax-aware diff rendering.
  - Add tests against fixture repositories with complex rename/binary cases.

## Open Questions

- What should the project/module name be?
- Should v1 initialize a git repository here, or should implementation move into an existing repository?
- Which theme variants should be prioritized after `tokyonight`, `tokyonight-storm`, `kanagawa-wave`, and `kanagawa-dragon`?

## Plan Handoff

### Source of Truth Docs

- `docs/STATE.md`
- `docs/ROADMAP.md`
- `docs/ARCHITECTURE.md`
- `docs/v1/designs/2026-06-11-v1-worktree-diff-tui.md`

### Scope for Planning

- Create one vertical slice: a Go CLI that opens a read-only TUI for current worktree changes.
- Include module scaffolding, git command adapter, view models, Bubble Tea app model, themed rendering, and focused tests.
- Planning should account for the current workspace not being a git repository.

### Fixed Constraints

- Language: Go.
- TUI stack: Bubble Tea v2, Bubbles, Lip Gloss.
- v1 behavior: read-only viewer.
- Data source: local `git` command output.
- No remote network calls in v1.

### Success Criteria

- Running the CLI inside a git repository opens a TUI with a changed-file list.
- Each changed file shows status plus added/deleted line counts where available.
- Selecting a file displays a scrollable diff preview.
- Dark, light, TokyoNight, and Kanagawa-style theme presets are applied through a central theme layer.
- Git command failures are visible in the TUI instead of crashing the terminal session.

### Non-Goals

- Mutating git state.
- Creating commits or pull requests.
- GitHub API integration.
- Supporting non-git directories beyond a clear error state.

### Open Questions

- Module name and binary name remain unset.
- Execution requires deciding whether to initialize git in this workspace.

### Suggested Validation

- Unit tests for git output parsing.
- Unit tests for theme token construction.
- Manual run in a fixture git repository with modified, added, deleted, renamed, and binary files.
- `go test ./...`.
- Manual terminal smoke test for navigation, scrolling, theme switching, and terminal recovery.

### Parallelization Hints

- Candidate write boundaries:
  - Git adapter and parsing.
  - Theme model and styles.
  - Bubble Tea model/update/view.
  - CLI entrypoint and wiring.
- Shared files to avoid touching in parallel:
  - `go.mod`
  - Main application wiring.
- Likely sequential dependencies:
  - Go module initialization before package work.
  - Git view models before full TUI rendering.
  - Theme tokens before final view styling.
