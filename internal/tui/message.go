package tui

import gitview "github.com/overthinker1127/tui-worktree/internal/git"

type reloadMsg struct {
	generation int
	snapshot   Snapshot
}

type autoRefreshMsg struct{}

type editorFinishedMsg struct {
	err error
}

type pullRequestFinishedMsg struct {
	err error
}

type mergeBranchFinishedMsg struct {
	request MergeRequest
	err     error
}

type deleteWorktreeFinishedMsg struct {
	worktree gitview.Worktree
	err      error
}

type diffLoadedMsg struct {
	revision    int
	worktree    string
	path        string
	diff        string
	diffYOffset int
}

type compareDiffLoadedMsg struct {
	generation int
	worktree   string
	path       string
	diff       string
}
