package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
)

type merge struct {
	targetList     list.Model
	source         gitview.Worktree
	request        MergeRequest
	confirmScrollX int
	merging        bool
}

func (m *merge) openTargetPicker(source gitview.Worktree, targetList list.Model) {
	m.source = source
	m.targetList = targetList
	m.targetList.Select(0)
}

func (m *merge) updateTargetList(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.targetList, cmd = m.targetList.Update(msg)
	return cmd
}

func (m merge) selectedTargetPath() string {
	target, ok := m.targetList.SelectedItem().(mergeTargetItem)
	if !ok {
		return ""
	}
	return target.worktree.Path
}

func (m merge) selectedRequest(fallbackSource gitview.Worktree) (MergeRequest, bool) {
	target, ok := m.targetList.SelectedItem().(mergeTargetItem)
	if !ok {
		return MergeRequest{}, false
	}
	request := MergeRequest{
		Source: m.source,
		Target: target.worktree,
	}
	if request.Source.Path == "" {
		request.Source = fallbackSource
	}
	return request, true
}

func (m *merge) openConfirm(request MergeRequest) {
	m.request = request
	m.confirmScrollX = 0
}

func (m *merge) restoreConfirm(request MergeRequest, merging bool, confirmScrollX int) {
	m.request = request
	m.source = request.Source
	m.merging = merging
	m.confirmScrollX = confirmScrollX
}

func (m *merge) cancelConfirm() {
	m.request = MergeRequest{}
	m.confirmScrollX = 0
}

func (m *merge) start() (MergeRequest, bool) {
	if m.request.Source.Path == "" || m.request.Target.Path == "" {
		return MergeRequest{}, false
	}
	m.merging = true
	return m.request, true
}

func (m *merge) finish() {
	m.merging = false
	m.cancelConfirm()
}

func (m *merge) clearTargetPicker() {
	m.source = gitview.Worktree{}
	m.targetList = list.Model{}
}
