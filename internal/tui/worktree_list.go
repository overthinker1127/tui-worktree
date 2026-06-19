package tui

import (
	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
	"github.com/overthinker1127/tui-worktree/internal/tui/components"
)

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
	list := l.component()
	if !list.Move(delta) {
		return WorktreeState{}, false
	}
	l.selectedWorktree = list.Selected
	return l.worktrees[l.selectedWorktree], true
}

func (l *worktreeList) selectIndex(index int) (WorktreeState, bool) {
	list := l.component()
	if !list.SelectIndex(index) {
		return WorktreeState{}, false
	}
	l.selectedWorktree = list.Selected
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
	return l.component().Offset(height)
}

func (l worktreeList) maxScrollX(styles theme.Styles, available int) int {
	return l.component().MaxScrollX(styles, available)
}

func (l worktreeList) render(styles theme.Styles, panel components.Panel, focused bool, width, height int) string {
	return l.component().Render(styles, panel, focused, width, height)
}

func (l worktreeList) component() components.WorktreeList {
	return components.WorktreeList{
		Items:    worktreeItems(l.worktrees),
		Selected: l.selectedWorktree,
		ScrollX:  l.worktreeScrollX,
	}
}

func worktreeItems(states []WorktreeState) []components.WorktreeItem {
	items := make([]components.WorktreeItem, len(states))
	for i, state := range states {
		items[i] = worktreeItem(state)
	}
	return items
}

func worktreeItem(state WorktreeState) components.WorktreeItem {
	worktree := state.Worktree
	return components.WorktreeItem{
		ID:        worktree.Path,
		Label:     worktreeLabel(worktree),
		Current:   worktree.Current,
		Protected: worktree.Protected,
		Error:     state.Error != nil,
	}
}

func renderWorktreeLine(styles theme.Styles, _ int, state WorktreeState) string {
	return components.RenderWorktreeItem(styles, worktreeItem(state))
}
