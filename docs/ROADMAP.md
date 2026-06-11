# Roadmap

## Current Track

- Active version: `v1`
- Exit criteria:
  - Go module and CLI entrypoint exist.
  - TUI shows current repository worktree changes in a GitHub PR-style files changed view.
  - Selected file diff preview is scrollable.
  - Theme presets are wired through a reusable theme model, including TokyoNight and Kanagawa-style presets.

## Upcoming Versions

- `v2`:
  - Goal: Add richer git interactions after the read-only viewer is stable.
  - Dependencies: v1 rendering, git command adapter, and theme boundaries.

## Deferred

- Candidate versions:
  - Staging controls.
  - Commit creation.
  - GitHub PR API integration.
  - go-git or libgit2-style internal parser migration.
- Open sequencing questions:
  - Whether to keep shelling out to `git` or migrate to a git library after MVP.
