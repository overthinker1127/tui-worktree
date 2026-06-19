package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type WorktreeItem struct {
	ID        string
	Label     string
	Current   bool
	Protected bool
	Error     bool
}

type WorktreeList struct {
	Items    []WorktreeItem
	Selected int
	ScrollX  int
}

func (l *WorktreeList) Move(delta int) bool {
	if len(l.Items) == 0 {
		return false
	}
	l.Selected += delta
	if l.Selected < 0 {
		l.Selected = len(l.Items) - 1
	}
	if l.Selected >= len(l.Items) {
		l.Selected = 0
	}
	return true
}

func (l *WorktreeList) SelectIndex(index int) bool {
	if index < 0 || index >= len(l.Items) {
		return false
	}
	l.Selected = index
	return true
}

func (l WorktreeList) Offset(height int) int {
	if len(l.Items) == 0 {
		return 0
	}
	visibleRows := l.VisibleRows(height)
	if l.Selected < visibleRows {
		return 0
	}
	offset := l.Selected - visibleRows + 1
	maxOffset := max(0, len(l.Items)-visibleRows)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (l WorktreeList) VisibleRows(height int) int {
	visibleRows := max(1, FrameInnerHeight(height))
	if len(l.Items) > visibleRows {
		return max(1, visibleRows-1)
	}
	return visibleRows
}

func (l WorktreeList) MaxScrollX(styles theme.Styles, available int) int {
	if len(l.Items) == 0 || l.Selected < 0 || l.Selected >= len(l.Items) {
		return 0
	}
	return max(0, lipgloss.Width(RenderWorktreeItem(styles, l.Items[l.Selected]))-available)
}

func (l WorktreeList) Render(styles theme.Styles, panel Panel, focused bool, width, height int) string {
	lines := make([]string, 0, len(l.Items))
	contentWidth := FrameInnerWidth(width)
	visibleRows := l.VisibleRows(height)
	offset := l.Offset(height)
	end := min(len(l.Items), offset+visibleRows)
	for i, item := range l.Items[offset:end] {
		index := offset + i
		line := RenderWorktreeItem(styles, item)
		if index == l.Selected {
			line = renderScrollableListRow(styles.FileSelected, IconSelected+" ", line, l.ScrollX, contentWidth, true)
		} else {
			line = renderScrollableListRow(panel.ListRowStyle(styles.FileItem), panel.ListFill("  "), line, l.ScrollX, contentWidth, false)
		}
		lines = append(lines, line)
	}
	if end < len(l.Items) {
		lines = append(lines, styles.Muted.Render(fmt.Sprintf("… %d more", len(l.Items)-end)))
	}
	innerHeight := FrameInnerHeight(height)
	title := fmt.Sprintf("[1]-%s %d worktrees", IconWorktree, len(l.Items))
	return panel.RenderPanel(width, height, focused, title, strings.Join(fillLines(lines, innerHeight), "\n"))
}

func RenderWorktreeItem(styles theme.Styles, item WorktreeItem) string {
	marker := " "
	if item.Current {
		marker = "•"
	}
	if item.Error {
		marker = "!"
	}
	if item.Protected {
		marker = IconProtected
	}
	return listStyle(styles, styles.Muted).Render(marker) +
		listFill(styles, " ") +
		listStyle(styles, styles.Muted).Render(IconBranch) +
		listFill(styles, " ") +
		listStyle(styles, styles.FileItem).Render(item.Label)
}

func renderScrollableListRow(style lipgloss.Style, prefix, content string, offset, width int, stripNestedANSI bool) string {
	if width <= 0 {
		return ""
	}
	offset = max(0, offset)
	prefixWidth := lipgloss.Width(prefix)
	contentWidth := max(1, width-prefixWidth)
	source := content
	if stripNestedANSI {
		source = ansi.Strip(content)
	}
	visible := ansi.Cut(source, offset, offset+contentWidth)
	row := prefix + visible
	rendered := style.Inline(true).MaxWidth(width).Render(row)
	if renderedWidth := lipgloss.Width(rendered); renderedWidth < width {
		rendered += style.Inline(true).Render(strings.Repeat(" ", width-renderedWidth))
	}
	return rendered
}

func listStyle(styles theme.Styles, style lipgloss.Style) lipgloss.Style {
	return style.Background(styles.Panel.GetBackground())
}

func listFill(styles theme.Styles, text string) string {
	return lipgloss.NewStyle().Background(styles.Panel.GetBackground()).Render(text)
}
