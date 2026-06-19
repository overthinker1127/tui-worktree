package tui

import gitview "github.com/overthinker1127/tui-worktree/internal/git"

type worktreeList struct {
	worktrees        []WorktreeState
	selectedWorktree int
	worktreeScrollX  int
}

func (l worktreeList) selectedWorktreeValue() gitview.Worktree {
	if len(l.worktrees) == 0 || l.selectedWorktree < 0 || l.selectedWorktree >= len(l.worktrees) {
		return gitview.Worktree{}
	}
	return l.worktrees[l.selectedWorktree].Worktree
}

func (l *worktreeList) normalize(fallbackChanges []gitview.FileChange, fallbackErr error) WorktreeState {
	if len(l.worktrees) == 0 {
		l.worktrees = []WorktreeState{{
			Worktree: gitview.Worktree{Path: ".", Branch: "current", Current: true},
			Changes:  fallbackChanges,
			Error:    fallbackErr,
		}}
		l.selectedWorktree = 0
		return l.worktrees[0]
	}
	if l.selectedWorktree < 0 || l.selectedWorktree >= len(l.worktrees) {
		l.selectedWorktree = 0
	}
	return l.worktrees[l.selectedWorktree]
}

func (l *worktreeList) move(delta int) (WorktreeState, bool) {
	if len(l.worktrees) == 0 {
		return WorktreeState{}, false
	}
	l.selectedWorktree += delta
	if l.selectedWorktree < 0 {
		l.selectedWorktree = len(l.worktrees) - 1
	}
	if l.selectedWorktree >= len(l.worktrees) {
		l.selectedWorktree = 0
	}
	return l.worktrees[l.selectedWorktree], true
}

func (l *worktreeList) selectIndex(index int) (WorktreeState, bool) {
	if index < 0 || index >= len(l.worktrees) {
		return WorktreeState{}, false
	}
	l.selectedWorktree = index
	return l.worktrees[index], true
}

func (l *worktreeList) remove(path string, fallbackChanges []gitview.FileChange, fallbackErr error) (WorktreeState, bool) {
	if path == "" {
		return WorktreeState{}, false
	}
	for i, state := range l.worktrees {
		if state.Worktree.Path != path {
			continue
		}
		l.worktrees = append(l.worktrees[:i], l.worktrees[i+1:]...)
		if l.selectedWorktree >= len(l.worktrees) {
			l.selectedWorktree = max(0, len(l.worktrees)-1)
		}
		return l.normalize(fallbackChanges, fallbackErr), true
	}
	return WorktreeState{}, false
}

func (l worktreeList) byPath(path string) (gitview.Worktree, bool) {
	if path == "" {
		return gitview.Worktree{}, false
	}
	for _, state := range l.worktrees {
		if state.Worktree.Path == path {
			return state.Worktree, true
		}
	}
	return gitview.Worktree{}, false
}

func (l worktreeList) offset(height int) int {
	if len(l.worktrees) == 0 {
		return 0
	}
	visibleRows := l.visibleRows(height)
	if l.selectedWorktree < visibleRows {
		return 0
	}
	offset := l.selectedWorktree - visibleRows + 1
	maxOffset := max(0, len(l.worktrees)-visibleRows)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (l worktreeList) visibleRows(height int) int {
	visibleRows := max(1, panelInnerHeight(height))
	if len(l.worktrees) > visibleRows {
		return max(1, visibleRows-1)
	}
	return visibleRows
}
