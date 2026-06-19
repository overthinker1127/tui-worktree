package tui

import (
	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/tui/components"
)

type overlap struct {
	cursor            int
	targets           []overlapTarget
	compareTarget     overlapTarget
	compareDiff       string
	compareLoading    bool
	compareYOffset    int
	compareXOffset    int
	compareGeneration int
}

type overlapTarget struct {
	Worktree gitview.Worktree
	Change   gitview.FileChange
}

func (o *overlap) openPicker(targets []overlapTarget) {
	o.targets = targets
	o.cursor = 0
}

func (o *overlap) closePicker() {
	o.targets = nil
	o.cursor = 0
}

func (o *overlap) moveCursor(delta int) {
	next := o.cursor + delta
	if next < 0 || next >= len(o.targets) {
		return
	}
	o.cursor = next
}

func (o *overlap) openCompare() (overlapTarget, bool) {
	if len(o.targets) == 0 || o.cursor < 0 || o.cursor >= len(o.targets) {
		return overlapTarget{}, false
	}
	o.compareTarget = o.targets[o.cursor]
	o.compareDiff = ""
	o.compareLoading = true
	o.compareYOffset = 0
	o.compareXOffset = 0
	o.compareGeneration++
	return o.compareTarget, true
}

func (o *overlap) closeCompare() {
	o.compareTarget = overlapTarget{}
	o.compareDiff = ""
	o.compareLoading = false
	o.compareYOffset = 0
	o.compareXOffset = 0
	o.compareGeneration++
}

func (o *overlap) scrollCompare(delta, maxYOffset int) {
	o.compareYOffset = clamp(o.compareYOffset+delta, 0, maxYOffset)
}

func (o *overlap) scrollCompareHorizontal(delta, maxXOffset int) {
	step := 6
	o.compareXOffset = clamp(o.compareXOffset+delta*step, 0, maxXOffset)
}

func (o *overlap) clampCompareOffsets(maxYOffset, maxXOffset int, softWrap bool) {
	o.compareYOffset = clamp(o.compareYOffset, 0, maxYOffset)
	if softWrap {
		o.compareXOffset = 0
		return
	}
	o.compareXOffset = clamp(o.compareXOffset, 0, maxXOffset)
}

func (o overlap) picker(selectedPath string) components.OverlapPicker {
	items := make([]components.OverlapItem, len(o.targets))
	for i, target := range o.targets {
		items[i] = components.OverlapItem{
			Label: worktreeLabel(target.Worktree),
			Path:  target.Worktree.Path,
		}
	}
	return components.OverlapPicker{
		SelectedPath: selectedPath,
		Items:        items,
		Cursor:       o.cursor,
	}
}

func (o overlap) compare(selectedWorktree gitview.Worktree, selectedFile gitview.FileChange, selectedDiff string) components.OverlapCompare {
	return components.OverlapCompare{
		LeftLabel:  worktreeLabel(selectedWorktree),
		RightLabel: worktreeLabel(o.compareTarget.Worktree),
		FilePath:   selectedFile.Path,
		LeftDiff:   selectedDiff,
		RightDiff:  o.compareDiff,
		RightPath:  o.compareTarget.Change.Path,
		Loading:    o.compareLoading,
		YOffset:    o.compareYOffset,
		XOffset:    o.compareXOffset,
	}
}
