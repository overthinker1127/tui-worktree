# tui-worktree

Read-only TUI for reviewing the current git worktree in a GitHub PR-style files changed view.

## Run

```bash
go run ./cmd/tui-worktree --repo . --theme tokyonight
```

Plan-compatible alias:

```bash
go run ./cmd/worktree-diff-tui --repo . --theme kanagawa
```

## Themes

- `dark`
- `light`
- `tokyonight`
- `tokyonight-night`
- `tokyonight-storm`
- `kanagawa`
- `kanagawa-wave`
- `kanagawa-dragon`

## Keys

- `j` / `down`: next file
- `k` / `up`: previous file
- `g` / `home`: first file
- `G` / `end`: last file
- `q` / `esc` / `ctrl+c`: quit

## Manual Smoke Test

```bash
tmp=$(mktemp -d)
cd "$tmp"
git init -b main
git config user.email smoke@example.com
git config user.name Smoke
printf "hello\n" > README.md
git add README.md
git commit -m init
printf "hello\nworld\n" > README.md
printf "new\n" > added.txt
go run /path/to/tui-worktree/cmd/tui-worktree --repo "$tmp" --theme tokyonight
```
