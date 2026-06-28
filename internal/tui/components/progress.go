package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func RenderProgressBar(trackStyle, fillStyle lipgloss.Style, width int, label string) string {
	if width <= 0 {
		return ""
	}
	label = ansi.Truncate(label, max(0, width-4), "")
	labelWidth := lipgloss.Width(label)
	barWidth := width
	if labelWidth > 0 && width >= labelWidth+5 {
		barWidth = width - labelWidth - 1
	} else {
		label = ""
		labelWidth = 0
	}
	innerWidth := max(1, barWidth-2)
	fillWidth := max(1, innerWidth/3)
	emptyWidth := max(0, innerWidth-fillWidth)

	bar := trackStyle.Inline(true).Render("[") +
		fillStyle.Inline(true).Render(strings.Repeat("=", fillWidth)) +
		trackStyle.Inline(true).Render(strings.Repeat("-", emptyWidth)+"]")
	if labelWidth > 0 {
		bar += trackStyle.Inline(true).Render(" " + label)
	}
	if renderedWidth := lipgloss.Width(bar); renderedWidth < width {
		bar += trackStyle.Inline(true).Render(strings.Repeat(" ", width-renderedWidth))
	}
	return ansi.Truncate(bar, width, "")
}
