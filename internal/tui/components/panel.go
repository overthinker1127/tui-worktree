package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type Panel struct {
	styles theme.Styles
}

func NewPanel(styles theme.Styles) Panel {
	return Panel{styles: styles}
}

func (r *Panel) SetStyles(styles theme.Styles) {
	r.styles = styles
}

func (r Panel) RenderPanel(width, height int, focused bool, title, content string) string {
	return r.RenderPanelWithFill(width, height, focused, title, content, r.panelStyle(focused))
}

func (r Panel) RenderPanelWithFill(width, height int, focused bool, title, content string, fillStyle lipgloss.Style) string {
	return r.RenderPanelWithFillStyles(width, height, focused, title, content, fillStyle, fillStyle)
}

func (r Panel) RenderPanelWithFillStyles(width, height int, focused bool, title, content string, lineFillStyle, emptyFillStyle lipgloss.Style) string {
	width = max(4, width)
	height = max(3, height)
	style := r.panelStyle(focused)
	innerWidth := FrameInnerWidth(width)
	innerHeight := FrameInnerHeight(height)
	border := style.GetBorderStyle()
	borderStyle := r.panelBorderStyle(style)
	lines := []string{r.RenderPanelTop(style, focused, title, width)}
	contentLines := strings.Split(content, "\n")
	for i := range innerHeight {
		fillStyle := lineFillStyle
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		} else {
			fillStyle = emptyFillStyle
		}
		lines = append(lines, r.renderPanelBodyLine(fillStyle, borderStyle, border, line, innerWidth))
	}
	lines = append(lines, r.renderPanelBottom(borderStyle, border, width))
	return strings.Join(fillLines(lines, height), "\n")
}

func (r Panel) RenderPanelTop(style lipgloss.Style, focused bool, title string, width int) string {
	border := style.GetBorderStyle()
	borderStyle := r.panelBorderStyle(style)
	label := r.renderPanelTitle(style, focused, title)
	innerWidth := FrameInnerWidth(width)
	if lipgloss.Width(label)+2 > innerWidth {
		prefix := "  "
		if focused {
			prefix = "● "
		}
		label = r.panelTitleStyle(style, focused).Render(ansi.Truncate(prefix+title, max(1, innerWidth-2), ""))
	}
	titlePad := r.panelFill(style, 1)
	titleSegment := titlePad + label + titlePad
	fillWidth := max(0, innerWidth-lipgloss.Width(titleSegment))
	return borderStyle.Render(border.TopLeft) +
		titleSegment +
		borderStyle.Render(strings.Repeat(border.Top, fillWidth)+border.TopRight)
}

func (r Panel) renderPanelBodyLine(style, borderStyle lipgloss.Style, border lipgloss.Border, line string, innerWidth int) string {
	line = ansi.Truncate(line, innerWidth, "")
	padding := r.panelFill(style, max(0, innerWidth-lipgloss.Width(line)))
	return borderStyle.Render(border.Left) + line + padding + borderStyle.Render(border.Right)
}

func (r Panel) renderPanelBottom(borderStyle lipgloss.Style, border lipgloss.Border, width int) string {
	innerWidth := FrameInnerWidth(width)
	return borderStyle.Render(border.BottomLeft + strings.Repeat(border.Bottom, innerWidth) + border.BottomRight)
}

func (r Panel) renderPanelTitle(style lipgloss.Style, focused bool, text string) string {
	if focused {
		return r.panelTitleStyle(style, focused).Render("● " + text)
	}
	return r.panelTitleStyle(style, focused).Render("  " + text)
}

func (r Panel) panelTitleStyle(style lipgloss.Style, focused bool) lipgloss.Style {
	titleStyle := lipgloss.NewStyle().
		Foreground(style.GetBorderTopForeground()).
		Background(style.GetBackground())
	if focused {
		titleStyle = titleStyle.Bold(true)
	}
	return titleStyle
}

func (r Panel) panelBorderStyle(style lipgloss.Style) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(style.GetBorderTopForeground()).
		Background(style.GetBackground())
}

func (r Panel) panelFill(style lipgloss.Style, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Background(style.GetBackground()).Render(strings.Repeat(" ", width))
}

func (r Panel) panelStyle(focused bool) lipgloss.Style {
	if focused {
		return r.styles.PanelFocused
	}
	return r.styles.Panel
}

func (r Panel) ListRowStyle(style lipgloss.Style) lipgloss.Style {
	return style.Background(r.styles.Panel.GetBackground())
}

func (r Panel) ListFill(text string) string {
	return lipgloss.NewStyle().Background(r.styles.Panel.GetBackground()).Render(text)
}

func fillLines(lines []string, height int) []string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		return lines[:height]
	}
	return lines
}
