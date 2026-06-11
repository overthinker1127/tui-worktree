# Architecture

## Purpose

- Records structural principles that are common to all versions.
- Detailed designs for each version are left in `docs/vN/designs/`.

## Shared Boundaries

- Core domains:
  - Git status and diff collection.
  - TUI state, navigation, and rendering.
  - Theme tokens and style application.
- External integrations:
  - Local `git` executable for MVP status, numstat, and diff reads.
- Data boundaries:
  - Git command output is parsed into typed file-change and diff view models.
  - Rendering consumes view models and theme tokens, not raw command output.

## Shared Constraints

- Security:
  - Treat repository paths and git output as untrusted text.
  - Do not execute arbitrary repository content.
- Reliability:
  - TUI should recover terminal state on exit and command failures.
  - Git command errors should render as actionable UI messages.
- Performance:
  - Initial target is responsive navigation for typical local worktrees.
  - Large diffs should be loaded and rendered through scrollable viewports.
- Operational limits:
  - MVP requires a local git repository and installed `git` executable.
  - No remote network calls in v1.
