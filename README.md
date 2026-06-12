# tui-worktree

Terminal UI for reviewing and managing linked Git worktrees. It shows worktrees, changed files, and diffs in a compact PR-style layout, with shortcuts for opening files, creating PRs/MRs, merging branches, deleting worktrees, and switching themes.

## Requirements

- Git
- Go 1.26.2 or newer
- A terminal with truecolor support
- Optional: [Task](https://taskfile.dev/) for the provided development commands
- Optional: `gh` or `glab` for PR/MR creation from the TUI

## Installation

Install from source:

```bash
go install github.com/overthinker1127/tui-worktree/cmd/tui-worktree@latest
```

From a local checkout:

```bash
task install
```

If you do not use Task:

```bash
go install ./cmd/tui-worktree
```

## Usage

Run against the current repository:

```bash
tui-worktree
```

Run against another repository:

```bash
tui-worktree --repo /path/to/repo
```

Choose a theme for the current run:

```bash
tui-worktree --theme kanagawa
```

Theme changes made from the TUI are saved to:

```text
~/.config/tui-worktree/config.json
```

The `--theme` flag overrides the saved theme for that run.

## Development

Run locally:

```bash
task run
```

Override the repository path:

```bash
task run REPO=/path/to/repo
```

Run checks:

```bash
task check
```

Without Task:

```bash
gofmt -w ./cmd ./internal
go mod tidy
go test ./...
go run ./cmd/tui-worktree --help
```

## Key Bindings

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

Worktree changes auto-refresh every 5 seconds.

## Editor Support

The `e` shortcut opens the selected file near the first changed line when the editor supports line arguments.

Supported editor styles include:

- Vim-style editors: `vi`, `vim`, `nvim`, `nano`, `micro`, `emacs`
- VS Code-style editors: `code`, `code-insiders`, `codium`, `vscodium`, `cursor`, `windsurf`
- Path-with-line editors: `zed`, `subl`, `hx`, `helix`
- JetBrains launchers: `idea`, `goland`, `webstorm`, `pycharm`, `clion`, `rider`

Unknown editors still open the selected file without a line hint.

## Safety Notes

- Protected branches and the current/default worktree are guarded against deletion.
- Delete and merge operations are explicit actions and should be reviewed before confirmation.
- PR/MR creation requires `gh` or `glab` to be authenticated.

## Smoke Test

Create a temporary repository with representative changes and open it in the TUI:

```bash
task smoke
```

Manual equivalent:

```bash
tmp=$(mktemp -d)
git -C "$tmp" init -b main
git -C "$tmp" config user.email smoke@example.com
git -C "$tmp" config user.name Smoke
printf "hello\n" > "$tmp/README.md"
printf "remove me\n" > "$tmp/deleted.txt"
printf "rename me\n" > "$tmp/old name.txt"
printf "\x00\x01\x02" > "$tmp/image.bin"
git -C "$tmp" add .
git -C "$tmp" commit -m init
printf "hello\nworld\n" > "$tmp/README.md"
printf "new\n" > "$tmp/added.txt"
git -C "$tmp" add added.txt
rm "$tmp/deleted.txt"
git -C "$tmp" mv "old name.txt" "new name.txt"
printf "\x03\x04" >> "$tmp/image.bin"
tui-worktree --repo "$tmp"
```

## License

This project is released under the license in [LICENSE](LICENSE).
