package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type OverlapItem struct {
	Label string
	Path  string
}

type OverlapPicker struct {
	SelectedPath string
	Items        []OverlapItem
	Cursor       int
}

func (p OverlapPicker) Render(appWidth, bodyHeight int, styles theme.Styles, overlayPanel lipgloss.Style) string {
	width := min(max(44, appWidth-12), 78)
	panel := overlayPanel.Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	title := styles.Title.
		Background(panel.GetBackground()).
		Width(contentWidth).
		Render(IconWarning + " Overlaps for " + p.SelectedPath)
	lines := []string{title}
	visibleRows := min(len(p.Items), max(1, bodyHeight-8))
	offset := p.offset(visibleRows)
	end := min(len(p.Items), offset+visibleRows)
	for i := offset; i < end; i++ {
		lines = append(lines, p.renderRow(i, contentWidth, styles, overlayPanel))
	}
	if len(p.Items) == 0 {
		lines = append(lines, styles.Muted.Background(panel.GetBackground()).Width(contentWidth).Render("No overlaps"))
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (p OverlapPicker) offset(visibleRows int) int {
	if visibleRows <= 0 || len(p.Items) <= visibleRows || p.Cursor < visibleRows {
		return 0
	}
	return min(p.Cursor-visibleRows+1, len(p.Items)-visibleRows)
}

func (p OverlapPicker) renderRow(index, width int, styles theme.Styles, overlayPanel lipgloss.Style) string {
	item := p.Items[index]
	style := styles.Diff.Background(overlayPanel.GetBackground())
	prefix := "  "
	if index == p.Cursor {
		style = styles.FileSelected
		prefix = IconSelected + " "
	}
	label := prefix + item.Label
	if item.Path != "" {
		label += "  " + item.Path
	}
	return RenderOverlayLine(style, width, label, 0)
}

type DiffTextRenderer func(diff string, width, height, yOffset, xOffset int) string

type OverlapCompare struct {
	LeftLabel  string
	RightLabel string
	FilePath   string
	LeftDiff   string
	RightDiff  string
	RightPath  string
	Loading    bool
	YOffset    int
	XOffset    int
}

func (c OverlapCompare) Render(
	background string,
	width, height int,
	styles theme.Styles,
	panel Panel,
	overlayPanel lipgloss.Style,
	renderDiffTextAt DiffTextRenderer,
) string {
	if width < 96 {
		return RenderOverlay(background, c.renderNarrowMessage(width, styles, overlayPanel), width, height)
	}
	return c.renderPanel(width, height, styles, panel, renderDiffTextAt)
}

func (c OverlapCompare) renderNarrowMessage(appWidth int, styles theme.Styles, overlayPanel lipgloss.Style) string {
	width := min(max(42, appWidth-8), 64)
	panel := overlayPanel.Width(width).Padding(1, 2)
	contentWidth := max(1, width-panel.GetHorizontalFrameSize())
	lines := []string{
		styles.Title.Background(panel.GetBackground()).Width(contentWidth).Render(IconWarning + " Compare"),
		styles.Diff.Background(panel.GetBackground()).Width(contentWidth).Render("Widen the terminal for side-by-side compare."),
	}
	return panel.Render(strings.Join(lines, "\n"))
}

func (c OverlapCompare) renderPanel(
	width, height int,
	styles theme.Styles,
	panel Panel,
	renderDiffTextAt DiffTextRenderer,
) string {
	width = max(4, width)
	height = max(4, height)
	title := fmt.Sprintf("[3]-Compare %s ↔ %s  %s", c.LeftLabel, c.RightLabel, c.FilePath)
	innerWidth := FrameInnerWidth(width)
	innerHeight := FrameInnerHeight(height)
	dividerWidth := 1
	columnWidth := max(1, (innerWidth-dividerWidth)/2)
	contentHeight := innerHeight
	leftDiff := c.LeftDiff
	if leftDiff == "" {
		leftDiff = fmt.Sprintf("No diff loaded for %s", c.FilePath)
	}
	rightDiff := c.RightDiff
	if c.Loading {
		rightDiff = "Loading overlap diff..."
	} else if rightDiff == "" {
		rightDiff = fmt.Sprintf("No diff loaded for %s", c.RightPath)
	}
	left := strings.Split(renderDiffTextAt(leftDiff, columnWidth, contentHeight, c.YOffset, c.XOffset), "\n")
	right := strings.Split(renderDiffTextAt(rightDiff, columnWidth, contentHeight, c.YOffset, c.XOffset), "\n")
	divider := styles.Muted.Background(styles.Diff.GetBackground()).Render("│")
	lines := make([]string, 0, contentHeight)
	for i := range contentHeight {
		leftLine, rightLine := "", ""
		if i < len(left) {
			leftLine = left[i]
		}
		if i < len(right) {
			rightLine = right[i]
		}
		lines = append(lines, leftLine+divider+rightLine)
	}
	return panel.RenderPanelWithFillStyles(width, height, true, title, strings.Join(lines, "\n"), styles.Diff, styles.Diff)
}
