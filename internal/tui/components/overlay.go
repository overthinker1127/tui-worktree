package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func RenderOverlayLine(style lipgloss.Style, width int, text string, offset int) string {
	if width <= 0 {
		return ""
	}
	offset = max(0, offset)
	rendered := style.Inline(true).MaxWidth(width).Render(ansi.Cut(text, offset, offset+width))
	if renderedWidth := lipgloss.Width(rendered); renderedWidth < width {
		rendered += style.Inline(true).Render(strings.Repeat(" ", width-renderedWidth))
	}
	return rendered
}

func RenderOverlay(background, foreground string, width, bodyHeight int) string {
	fgWidth := lipgloss.Width(foreground)
	fgHeight := lipgloss.Height(foreground)
	x, y := OverlayPosition(foreground, width, bodyHeight)

	bgLines := strings.Split(background, "\n")
	fgLines := strings.Split(foreground, "\n")
	if len(bgLines) < y+fgHeight {
		bgLines = append(bgLines, make([]string, y+fgHeight-len(bgLines))...)
	}
	for i, line := range fgLines {
		bgIndex := y + i
		bgLine := bgLines[bgIndex]
		left := ansi.Cut(bgLine, 0, x)
		right := ansi.Cut(bgLine, x+fgWidth, lipgloss.Width(bgLine))
		bgLines[bgIndex] = left + line + right
	}
	return strings.Join(bgLines, "\n")
}

func OverlayPosition(foreground string, width, bodyHeight int) (int, int) {
	fgWidth := lipgloss.Width(foreground)
	fgHeight := lipgloss.Height(foreground)
	x := max(0, (width-fgWidth)/2)
	y := max(0, (bodyHeight-fgHeight)/3)
	return x, y
}
