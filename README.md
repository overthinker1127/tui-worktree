# tui-worktree

Read-only TUI for reviewing the current git worktree in a GitHub PR-style files changed view.

## Run

```bash
task run
```

Override repo or theme:

```bash
task run REPO=/path/to/repo THEME=kanagawa
```

Plan-compatible alias:

```bash
task run-alias THEME=catppuccin
```

Verification:

```bash
task check
```

## Themes

- `tokyonight`
- `tokyonight-night`
- `tokyonight-storm`
- `kanagawa`
- `kanagawa-wave`
- `kanagawa-dragon`
- `catppuccin`
- `catppuccin-mocha`
- `catppuccin-macchiato`
- `gruvbox`
- `gruvbox-dark`
- `solarized`
- `solarized-dark`
- `nord`
- `dracula`
- `rose-pine`
- `rose-pine-moon`
- `one-dark`
- `vscode`
- `vscode-dark`
- `monokai`
- `everforest`
- `ayu`
- `ayu-mirage`

## Keys

- `j` / `down`: next file
- `k` / `up`: previous file
- `g` / `home`: first file
- `G` / `end`: last file
- `r`: refresh worktree changes
- `t`: open theme picker
- `?`: toggle help
- mouse click: select files or theme entries
- `q` / `esc` / `ctrl+c`: quit

## Manual Smoke Test

```bash
tmp=$(mktemp -d)
git -C "$tmp" init -b main
git -C "$tmp" config user.email smoke@example.com
git -C "$tmp" config user.name Smoke
printf "hello\n" > "$tmp/README.md"
git -C "$tmp" add README.md
git -C "$tmp" commit -m init
printf "hello\nworld\n" > "$tmp/README.md"
printf "new\n" > "$tmp/added.txt"
task run REPO="$tmp" THEME=tokyonight
```

Or use the built-in smoke task:

```bash
task smoke
```
