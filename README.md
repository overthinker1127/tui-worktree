# tui-worktree

## Project overview

`tui-worktree` is a terminal UI for reviewing and managing linked Git worktrees. It presents worktrees, changed files, and diffs in a compact PR-style layout so you can move between worktrees, inspect changes, open files, and run common branch actions without leaving the terminal.

The project is written in Go and uses Bubble Tea, Bubbles, and Lip Gloss for the TUI.

## Features

- Browse linked Git worktrees and their changed files
- Review file diffs with wrapping, line numbers, syntax-aware highlighting, and scroll preservation during refreshes
- Auto-refresh worktree changes every 5 seconds
- Open the selected file in `$EDITOR` near the first changed line when the editor supports line arguments
- Create PRs/MRs through `gh` or `glab`
- Merge a selected worktree branch into another worktree branch
- Delete non-protected worktrees and branches after confirmation
- Switch and persist themes from inside the TUI
- Protect current/default worktrees and protected branch names from destructive actions

## Installation

Requirements:

- Git
- Homebrew, or Go 1.26.2 or newer for `go install`
- A terminal with truecolor support
- Optional: `gh` or `glab` for PR/MR creation

Install with Homebrew:

```bash
brew install overthinker1127/tap/tui-worktree
```

Or install with Go:

```bash
go install github.com/overthinker1127/tui-worktree/cmd/tui-worktree@latest
```

When installing with Go, make sure your Go binary directory is on `PATH`. Common locations are `$(go env GOPATH)/bin` or `GOBIN` when configured.

## Usage examples

Open the current repository:

```bash
tui-worktree
```

Open another repository:

```bash
tui-worktree --repo /path/to/repo
```

Use a specific theme for one run:

```bash
tui-worktree --theme kanagawa
```

Show help:

```bash
tui-worktree --help
```

Common key bindings:

- `1` / `2` / `3`: focus worktrees, files, or diff panel
- `tab` / `shift+tab`: next or previous worktree
- `j` / `down`: next item or scroll diff down
- `k` / `up`: previous item or scroll diff up
- `g` / `home`: first file, or top of diff when diff is focused
- `G` / `end`: last file, or bottom of diff when diff is focused
- `e`: open selected file in `$EDITOR`
- `p`: create a PR/MR with `gh` or `glab`
- `d`: delete selected worktree and branch after confirmation
- `m`: merge the selected worktree branch into a selected target branch
- `w`: toggle diff wrap
- `n`: toggle diff line numbers
- `t`: open theme picker
- `q` / `ctrl+c`: quit

## Configuration

The app stores user configuration at:

```text
~/.config/tui-worktree/config.json
```

Theme changes made from the TUI are saved there automatically. The `--theme` flag overrides the saved theme for the current run only.

Editor integration uses `$EDITOR`. Known editor families receive line hints for the first changed line:

- Vim-style editors: `vi`, `vim`, `nvim`, `nano`, `micro`, `emacs`
- VS Code-style editors: `code`, `code-insiders`, `codium`, `vscodium`, `cursor`, `windsurf`
- Path-with-line editors: `zed`, `subl`, `hx`, `helix`
- JetBrains launchers: `idea`, `goland`, `webstorm`, `pycharm`, `clion`, `rider`

Unknown editors still open the selected file without a line hint.

## Development

Run the app from source:

```bash
go run ./cmd/tui-worktree --repo /path/to/repo
```

Format, tidy, and test:

```bash
gofmt -w ./cmd ./internal
go mod tidy
go test ./...
go vet ./...
```

The compatibility command is available at:

```bash
go run ./cmd/worktree-diff-tui --repo /path/to/repo
```

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
