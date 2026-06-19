package components

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type FileItem struct {
	ID         string
	Path       string
	StatusIcon string
	Additions  int
	Deletions  int
	Binary     bool
}

type FileBadgeFunc func(FileItem, color.Color) string

type FileList struct {
	Items    []FileItem
	Selected int
	ScrollX  int
	Filter   string
}

func (l FileList) SelectedItem() (FileItem, bool) {
	items := l.VisibleItems()
	if len(items) == 0 || l.Selected < 0 || l.Selected >= len(items) {
		return FileItem{}, false
	}
	return items[l.Selected], true
}

func (l FileList) VisibleItems() []FileItem {
	if l.Filter == "" {
		return l.Items
	}
	filter := strings.ToLower(l.Filter)
	visible := make([]FileItem, 0, len(l.Items))
	for _, item := range l.Items {
		if strings.Contains(strings.ToLower(item.Path), filter) {
			visible = append(visible, item)
		}
	}
	return visible
}

func (l *FileList) RestoreSelected(id string, fallbackIndex int) {
	items := l.VisibleItems()
	if id != "" {
		for index, item := range items {
			if item.ID == id {
				l.Selected = index
				return
			}
		}
	}
	l.Selected = min(fallbackIndex, max(0, len(items)-1))
}

func (l *FileList) MoveSelection(delta int) bool {
	items := l.VisibleItems()
	if len(items) == 0 {
		return false
	}
	l.Selected += delta
	if l.Selected < 0 {
		l.Selected = 0
	}
	if l.Selected >= len(items) {
		l.Selected = len(items) - 1
	}
	return true
}

func (l *FileList) ClearFilter() {
	l.Filter = ""
}

func (l *FileList) RemoveFilterRune() bool {
	if l.Filter == "" {
		return false
	}
	runes := []rune(l.Filter)
	l.Filter = string(runes[:len(runes)-1])
	return true
}

func (l *FileList) AppendFilter(text string) {
	l.Filter += text
}

func (l FileList) Offset(height int) int {
	items := l.VisibleItems()
	if len(items) == 0 {
		return 0
	}
	visibleRows := l.VisibleRows(height)
	if l.Selected < visibleRows {
		return 0
	}
	offset := l.Selected - visibleRows + 1
	maxOffset := max(0, len(items)-visibleRows)
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (l FileList) VisibleRows(height int) int {
	items := l.VisibleItems()
	visibleRows := max(1, FrameInnerHeight(height))
	if len(items) > visibleRows {
		return max(1, visibleRows-1)
	}
	return visibleRows
}

func (l FileList) MaxScrollX(styles theme.Styles, available int, badge FileBadgeFunc) int {
	item, ok := l.SelectedItem()
	if !ok {
		return 0
	}
	line := RenderFileItem(styles, item, l.Filter, styles.Panel.GetBackground(), badge(item, styles.Panel.GetBackground()))
	return max(0, lipgloss.Width(line)-available)
}

func (l FileList) Render(styles theme.Styles, panel Panel, focused bool, width, height int, badge FileBadgeFunc, overlapCount int) string {
	items := l.VisibleItems()
	lines := make([]string, 0, len(items))
	contentWidth := FrameInnerWidth(width)
	visibleRows := l.VisibleRows(height)
	offset := l.Offset(height)
	end := min(len(items), offset+visibleRows)
	for i, item := range items[offset:end] {
		index := offset + i
		if index == l.Selected {
			line := l.renderLine(styles, item, styles.FileSelected.GetBackground(), contentWidth, badge)
			line = renderScrollableListRow(styles.FileSelected, IconSelected+" ", line, l.ScrollX, contentWidth, false)
			lines = append(lines, line)
		} else {
			line := l.renderLine(styles, item, styles.Panel.GetBackground(), contentWidth, badge)
			line = renderScrollableListRow(panel.ListRowStyle(styles.FileItem), panel.ListFill("  "), line, l.ScrollX, contentWidth, false)
			lines = append(lines, line)
		}
	}
	if len(l.Items) == 0 {
		lines = append(lines, panel.ListRowStyle(styles.Muted).Render("No changed files"))
	} else if len(items) == 0 {
		lines = append(lines, panel.ListRowStyle(styles.Muted).Render("No matching files"))
	} else if end < len(items) {
		lines = append(lines, panel.ListRowStyle(styles.Muted).Render(fmt.Sprintf("… %d more", len(items)-end)))
	}
	innerHeight := FrameInnerHeight(height)
	return panel.RenderPanel(width, height, focused, l.title(len(items), overlapCount), strings.Join(fillLines(lines, innerHeight), "\n"))
}

func (l FileList) RenderFilter(styles theme.Styles, overlayPanel lipgloss.Style, appWidth int) string {
	width := min(max(32, appWidth-20), 56)
	panel := overlayPanel.Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	title := styles.Title.
		Background(panel.GetBackground()).
		Width(contentWidth).
		Render(IconFile + " Filters")
	query := styles.Diff.
		Background(panel.GetBackground()).
		Width(contentWidth).
		Render(l.Filter)
	return panel.Render(lipgloss.JoinVertical(lipgloss.Left, title, query))
}

func (l FileList) renderLine(styles theme.Styles, item FileItem, background color.Color, rowWidth int, badge FileBadgeFunc) string {
	if l.ScrollX > 0 || l.Filter != "" {
		return RenderFileItem(styles, item, l.Filter, background, badge(item, background))
	}
	prefixWidth := lipgloss.Width(IconSelected + " ")
	contentWidth := max(1, rowWidth-prefixWidth)
	return RenderFileItemWithinWidth(styles, item, l.Filter, background, contentWidth, badge(item, background))
}

func (l FileList) title(visibleCount, overlapCount int) string {
	if l.Filter != "" {
		return fmt.Sprintf("[2]-%s %d filtered [Esc]", IconFile, visibleCount)
	}
	title := fmt.Sprintf("[2]-%s %d files", IconFile, visibleCount)
	if overlapCount > 0 {
		title += fmt.Sprintf("  %d overlaps", overlapCount)
	}
	return title
}

func RenderFileItem(styles theme.Styles, item FileItem, filter string, background color.Color, suffixParts ...string) string {
	suffix := ""
	if len(suffixParts) > 0 {
		suffix = suffixParts[0]
	}
	return listStyleWithBackground(styles.Muted, background).Render(item.StatusIcon) +
		listFillWithBackground(background, " ") +
		renderFilteredPathWithBackground(styles, item.Path, filter, background) +
		fileLineCounts(styles, item, background) +
		suffix
}

func RenderFileItemWithinWidth(styles theme.Styles, item FileItem, filter string, background color.Color, width int, suffix string) string {
	status := listStyleWithBackground(styles.Muted, background).Render(item.StatusIcon)
	space := listFillWithBackground(background, " ")
	counts := fileLineCounts(styles, item, background)
	pathWidth := max(0, width-lipgloss.Width(status)-lipgloss.Width(space)-lipgloss.Width(counts)-lipgloss.Width(suffix))
	path := middleEllipsizePath(item.Path, pathWidth)
	return status + space + renderFilteredPathWithBackground(styles, path, filter, background) + counts + suffix
}

func fileLineCounts(styles theme.Styles, item FileItem, background color.Color) string {
	if item.Binary {
		return listFillWithBackground(background, " ") +
			listStyleWithBackground(styles.Muted, background).Render(IconBinary) +
			listFillWithBackground(background, " ") +
			listStyleWithBackground(styles.Muted, background).Render("binary")
	}
	if item.Additions == 0 && item.Deletions == 0 {
		return ""
	}
	return listFillWithBackground(background, " ") +
		listStyleWithBackground(styles.Added, background).Render(fmt.Sprintf("+%d", item.Additions)) +
		listFillWithBackground(background, " ") +
		listStyleWithBackground(styles.Deleted, background).Render(fmt.Sprintf("-%d", item.Deletions))
}

func middleEllipsizePath(path string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= width {
		return path
	}
	if width == 1 {
		return "…"
	}
	available := width - 1
	prefixWidth := max(1, available/3)
	suffixWidth := max(0, available-prefixWidth)
	if suffixWidth == 0 {
		return ansi.Cut(path, 0, prefixWidth) + "…"
	}
	return ansi.Cut(path, 0, prefixWidth) + "…" + ansi.Cut(path, lipgloss.Width(path)-suffixWidth, lipgloss.Width(path))
}

func renderFilteredPathWithBackground(styles theme.Styles, path, filter string, background color.Color) string {
	if filter == "" {
		return listStyleWithBackground(styles.FileItem, background).Render(path)
	}
	index := strings.Index(strings.ToLower(path), strings.ToLower(filter))
	if index < 0 {
		return listStyleWithBackground(styles.FileItem, background).Render(path)
	}
	end := index + len(filter)
	base := listStyleWithBackground(styles.FileItem, background)
	match := base.Bold(true)
	return base.Render(path[:index]) + match.Render(path[index:end]) + base.Render(path[end:])
}

func listStyleWithBackground(style lipgloss.Style, background color.Color) lipgloss.Style {
	return style.Background(background)
}

func listFillWithBackground(background color.Color, text string) string {
	return lipgloss.NewStyle().Background(background).Render(text)
}
