package tui

import (
	"context"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
)

type dependencies struct {
	loadDiff          func(context.Context, string, gitview.FileChange) string
	deleteWorktree    func(context.Context, gitview.Worktree) error
	reload            func(context.Context, string) Snapshot
	saveTheme         func(string) error
	saveTransparent   func(bool) error
	findForgeCLI      func() (string, bool)
	createPullRequest func(context.Context, PullRequestRequest) error
	mergeBranch       func(context.Context, MergeRequest) error
}
