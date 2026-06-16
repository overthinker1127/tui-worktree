# Third-party licenses

This directory contains license notices for runtime Go dependencies used by
`tui-worktree`.

The files under `licenses/` are generated with:

```bash
go-licenses save ./cmd/tui-worktree \
  --ignore=github.com/overthinker1127/tui-worktree \
  --save_path=third_party/licenses
```

