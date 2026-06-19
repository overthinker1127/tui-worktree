package command

import gitview "github.com/overthinker1127/tui-worktree/internal/git"

type PullRequestRequest struct {
	CLI         string
	WorktreeDir string
	Branch      string
	Title       string
	Body        string
}

type MergeRequest struct {
	Source gitview.Worktree
	Target gitview.Worktree
}
