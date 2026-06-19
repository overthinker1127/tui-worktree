package tui

import (
	"slices"
	"strings"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
	"github.com/overthinker1127/tui-worktree/internal/tui/components"
)

type merge struct {
	source  gitview.Worktree
	request MergeRequest
	picker  components.MergePicker
}

func (m *merge) openTargetPicker(source gitview.Worktree, targets []components.MergeTarget, selectedID string, styles theme.Styles, screenWidth, bodyHeight int) {
	m.source = source
	m.picker.Open(worktreeLabel(source), targets, selectedID, styles, screenWidth, bodyHeight)
}

func (m merge) selectedTargetPath() string {
	return m.picker.SelectedID()
}

func (m *merge) openConfirm(request MergeRequest) {
	m.request = request
}

func (m *merge) cancelConfirm() {
	m.request = MergeRequest{}
}

func (m *merge) clearTargetPicker() {
	m.source = gitview.Worktree{}
	m.picker.Clear()
}

func (m *merge) finish() {
	m.cancelConfirm()
}

func mergeTargets(source gitview.Worktree, worktrees []WorktreeState, defaultBranch string) []components.MergeTarget {
	candidates := make([]mergeTargetCandidate, 0, len(worktrees))
	for _, state := range worktrees {
		worktree := state.Worktree
		if worktree.Path == "" || worktree.Branch == "" || worktree.Branch == "detached" || worktree.Path == source.Path {
			continue
		}
		candidates = append(candidates, mergeTargetCandidate{worktree: worktree})
	}
	for i, candidate := range candidates {
		if candidate.worktree.DefaultBranch || candidate.worktree.Branch == defaultBranch {
			candidates[0], candidates[i] = candidates[i], candidates[0]
			break
		}
	}
	targets := make([]components.MergeTarget, len(candidates))
	for i, candidate := range candidates {
		worktree := candidate.worktree
		targets[i] = components.MergeTarget{
			ID:          worktree.Path,
			Title:       worktreeLabel(worktree),
			Description: worktree.Path,
			Default:     worktree.DefaultBranch,
		}
	}
	return targets
}

type mergeTargetCandidate struct {
	worktree gitview.Worktree
}

func selectedMergeRequest(source gitview.Worktree, targets []WorktreeState, selectedTargetPath string) (MergeRequest, bool) {
	if source.Path == "" || selectedTargetPath == "" {
		return MergeRequest{}, false
	}
	index := slices.IndexFunc(targets, func(state WorktreeState) bool {
		return state.Worktree.Path == selectedTargetPath
	})
	if index < 0 {
		return MergeRequest{}, false
	}
	return MergeRequest{
		Source: source,
		Target: targets[index].Worktree,
	}, true
}

func mergeConfirmText(request MergeRequest) (string, string) {
	source := worktreeLabel(request.Source)
	target := worktreeLabel(request.Target)
	return "Merge " + source + " into " + target, strings.Join([]string{
		"Source: " + source,
		"Target: " + target,
		"Target worktree will be updated first.",
		"Dirty files and conflicts will be checked before merging.",
	}, "\n")
}
