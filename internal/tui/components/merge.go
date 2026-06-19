package components

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type MergePickerAction int

const (
	MergePickerNone MergePickerAction = iota
	MergePickerCancel
	MergePickerQuit
	MergePickerConfirm
)

type MergeTarget struct {
	ID          string
	Title       string
	Description string
	Default     bool
}

type MergePicker struct {
	source string
	list   list.Model
}

func (p *MergePicker) Open(source string, targets []MergeTarget, selectedID string, styles theme.Styles, screenWidth, bodyHeight int) {
	items := make([]list.Item, len(targets))
	for i, target := range targets {
		items[i] = mergeTargetItem{target: target}
	}
	p.source = source
	p.list = newMergeTargetList(items, styles, screenWidth, bodyHeight)
	p.list.Select(0)
	p.selectID(selectedID)
}

func (p *MergePicker) Clear() {
	*p = MergePicker{}
}

func (p *MergePicker) HandleKey(msg tea.KeyPressMsg) (MergePickerAction, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return MergePickerQuit, nil
	case "esc", "m":
		return MergePickerCancel, nil
	case "enter":
		return MergePickerConfirm, nil
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return MergePickerNone, cmd
}

func (p MergePicker) SelectedID() string {
	target, ok := p.list.SelectedItem().(mergeTargetItem)
	if !ok {
		return ""
	}
	return target.target.ID
}

func (p MergePicker) Render(screenWidth, bodyHeight int, styles theme.Styles, panel lipgloss.Style) string {
	targets := p.list
	width := min(max(40, screenWidth-12), 70)
	panel = panel.Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	height := min(max(6, len(targets.Items())*2+2), max(6, bodyHeight-5))
	targets.SetSize(contentWidth, height)
	source := renderLine(styles.Muted, contentWidth, "From: "+p.source+"  >  Target: "+selectedMergeTargetLabel(targets), 0)
	return panel.Render(lipgloss.JoinVertical(lipgloss.Left, source, targets.View()))
}

func (p *MergePicker) selectID(id string) {
	if id == "" {
		return
	}
	items := p.list.Items()
	if index := slices.IndexFunc(items, func(item list.Item) bool {
		target, ok := item.(mergeTargetItem)
		return ok && target.target.ID == id
	}); index >= 0 {
		p.list.Select(index)
	}
}

func selectedMergeTargetLabel(targets list.Model) string {
	target, ok := targets.SelectedItem().(mergeTargetItem)
	if !ok {
		return ""
	}
	return target.target.Title
}

func newMergeTargetList(items []list.Item, styles theme.Styles, screenWidth, bodyHeight int) list.Model {
	width := min(max(34, screenWidth-12), 64)
	height := min(max(6, len(items)*2+2), max(6, bodyHeight-4))
	targets := list.New(items, mergeTargetDelegate{styles: styles}, width, height)
	targets.Title = IconMerge + " Merge into"
	targets.SetFilteringEnabled(false)
	targets.SetShowFilter(false)
	targets.SetShowStatusBar(false)
	targets.SetShowHelp(false)
	targets.SetShowPagination(false)
	targets.DisableQuitKeybindings()
	targets.Styles.TitleBar = lipgloss.NewStyle().Background(styles.Panel.GetBackground())
	targets.Styles.Title = styles.Title.Background(styles.Panel.GetBackground())
	targets.Styles.NoItems = styles.Muted.Background(styles.Panel.GetBackground())
	return targets
}

type mergeTargetDelegate struct {
	styles theme.Styles
}

func (d mergeTargetDelegate) Height() int {
	return 2
}

func (d mergeTargetDelegate) Spacing() int {
	return 0
}

func (d mergeTargetDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}

func (d mergeTargetDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	target, ok := item.(mergeTargetItem)
	if !ok {
		return
	}
	background := d.styles.Panel.GetBackground()
	style := d.styles.FileItem.Background(background).Inline(true)
	if index == m.Index() {
		style = d.styles.FileSelected.Background(background).Inline(true)
	}
	width := max(1, m.Width())
	title := IconSelected + " " + target.Title()
	if index != m.Index() {
		title = "  " + target.Title()
	}
	description := "  " + target.Description()
	lines := []string{
		style.Width(width).Render(ansi.Truncate(title, width, "")),
		d.styles.Muted.Background(background).Width(width).Render(ansi.Truncate(description, width, "")),
	}
	_, _ = fmt.Fprint(w, strings.Join(lines, "\n"))
}

type mergeTargetItem struct {
	target MergeTarget
}

func (i mergeTargetItem) FilterValue() string {
	return i.target.Title
}

func (i mergeTargetItem) Title() string {
	title := IconBranch + " " + i.target.Title
	if i.target.Default {
		title += " (default)"
	}
	return title
}

func (i mergeTargetItem) Description() string {
	return i.target.Description
}
