# tui-worktree

Read-only TUI for reviewing linked git worktrees in a GitHub PR-style files changed view.

## Run

```bash
task run
```

Override repo or theme:

```bash
task run REPO=/path/to/repo THEME=kanagawa
```

Theme changes made from the TUI are saved to:

```text
~/.config/tui-worktree/config.json
```

`--theme` still overrides the saved theme for that run.

Install:

```bash
task install
```

Compatibility alias:

```bash
task run-legacy THEME=catppuccin
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

- `1`-`9`: jump to worktree
- `tab` / `shift+tab`: next or previous worktree
- `j` / `down`: next file
- `k` / `up`: previous file
- `g` / `home`: first file
- `G` / `end`: last file
- `r`: refresh worktree changes
- `t`: open theme picker
- `?`: toggle help
- `q` / `esc` / `ctrl+c`: quit

## Manual Smoke Test

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
task run REPO="$tmp" THEME=tokyonight
```

Or use the built-in smoke task:

```bash
task smoke
```
