package components

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/overthinker1127/tui-worktree/internal/theme"
)

type Diff struct {
	viewport        viewport.Model
	lines           []string
	content         string
	contentKey      string
	styles          theme.Styles
	showLineNumbers bool
}

func NewDiff(styles theme.Styles) Diff {
	vp := viewport.New()
	vp.SoftWrap = true
	diff := Diff{
		viewport:        vp,
		styles:          styles,
		showLineNumbers: true,
	}
	diff.refreshViewportStyles()
	return diff
}

func (d *Diff) SetStyles(styles theme.Styles) {
	d.styles = styles
	d.refreshViewportStyles()
}

func (d *Diff) SetSize(width, height int) {
	d.viewport.SetWidth(max(1, width))
	d.viewport.SetHeight(max(1, height))
	d.refreshViewportStyles()
}

func (d *Diff) SetContent(key, diff string) bool {
	if key == d.contentKey && diff == d.content {
		return false
	}
	d.contentKey = key
	d.content = diff
	d.lines = strings.Split(diff, "\n")
	d.refreshViewportStyles()
	d.viewport.SetContent(diff)
	return true
}

func (d Diff) Render(title string, panel Panel, focused bool, width, height int) string {
	content := d.RenderContent()
	return panel.RenderPanelWithFillStyles(width, height, focused, title, content, d.styles.Diff, d.styles.Diff)
}

func (d *Diff) ToggleWrap() bool {
	if d.viewport.SoftWrap {
		d.viewport.SoftWrap = false
		return false
	}
	d.viewport.SetXOffset(0)
	d.viewport.SoftWrap = true
	return true
}

func (d *Diff) ToggleLineNumbers() bool {
	d.showLineNumbers = !d.showLineNumbers
	d.SetXOffset(clampDiff(d.XOffset(), 0, d.MaxXOffset()))
	return d.showLineNumbers
}

func (d Diff) SoftWrap() bool {
	return d.viewport.SoftWrap
}

func (d Diff) ShowLineNumbers() bool {
	return d.showLineNumbers
}

func (d Diff) Width() int {
	return d.viewport.Width()
}

func (d Diff) Height() int {
	return d.viewport.Height()
}

func (d Diff) XOffset() int {
	return d.viewport.XOffset()
}

func (d *Diff) SetXOffset(offset int) {
	d.viewport.SetXOffset(offset)
}

func (d Diff) YOffset() int {
	return d.viewport.YOffset()
}

func (d *Diff) SetYOffset(offset int) {
	d.viewport.SetYOffset(offset)
}

func (d *Diff) GotoTop() {
	d.viewport.GotoTop()
}

func (d *Diff) GotoBottom() {
	d.viewport.GotoBottom()
}

func (d *Diff) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return cmd
}

func (d Diff) MaxXOffset() int {
	return max(0, d.maxLineWidth()-d.TextWidth(d.viewport.Width()))
}

func (d Diff) DisplayLineCount(diff string, width int) int {
	if diff == "" {
		return 1
	}
	numbered := numberedDiffLines(strings.Split(diff, "\n"))
	if !d.viewport.SoftWrap {
		return len(numbered)
	}
	textWidth := d.TextWidth(width)
	count := 0
	for _, line := range numbered {
		count += len(wrapDisplaySegments(line.text, textWidth))
	}
	return max(1, count)
}

func (d Diff) MaxTextLineWidth(diff string) int {
	width := 0
	for _, line := range strings.Split(diff, "\n") {
		width = max(width, lipgloss.Width(line))
	}
	return width
}

func (d Diff) maxLineWidth() int {
	width := 0
	for _, line := range d.lines {
		width = max(width, lipgloss.Width(line))
	}
	return width
}

func (d *Diff) refreshViewportStyles() {
	d.viewport.Style = d.styles.Diff
	lines := d.lines
	d.viewport.StyleLineFunc = func(index int) lipgloss.Style {
		width := d.viewport.Width()
		if index < 0 || index >= len(lines) {
			return d.styles.Diff.Inline(true).Width(width)
		}
		return d.lineStyle(lines[index]).Inline(true).Width(width)
	}
}

func (d Diff) RenderContent() string {
	width := d.viewport.Width()
	height := d.viewport.Height()
	if width <= 0 || height <= 0 {
		return ""
	}
	textWidth := d.TextWidth(width)
	if d.viewport.SoftWrap {
		return d.renderWrappedViewport(width, textWidth, height)
	}
	return d.renderUnwrappedViewport(width, textWidth, height)
}

func (d Diff) renderWrappedViewport(width, textWidth, height int) string {
	lines := make([]string, 0, height)
	offset := d.viewport.YOffset()
	seen := 0
	for _, line := range d.numberedLines() {
		style := d.lineStyle(line.text)
		highlight := shouldHighlightDiffSyntaxLine(line)
		segments := wrapDisplaySegments(line.text, textWidth)
		for segmentIndex, segment := range segments {
			if seen >= offset {
				lines = append(lines, d.renderSegment(style, d.lineNumberGutter(line, segmentIndex > 0), segment, width, textWidth, highlight))
				if len(lines) == height {
					return strings.Join(lines, "\n")
				}
			}
			seen++
		}
	}
	return strings.Join(fillStyledLines(lines, height, d.renderSegment(d.styles.Diff, "", "", width, textWidth, false)), "\n")
}

func (d Diff) renderUnwrappedViewport(width, textWidth, height int) string {
	lines := make([]string, 0, height)
	offset := d.viewport.YOffset()
	xOffset := d.viewport.XOffset()
	numbered := d.numberedLines()
	for i := offset; i < len(numbered) && len(lines) < height; i++ {
		line := numbered[i]
		segment := ansi.Cut(line.text, xOffset, xOffset+textWidth)
		lines = append(lines, d.renderSegment(d.lineStyle(line.text), d.lineNumberGutter(line, false), segment, width, textWidth, shouldHighlightDiffSyntaxLine(line)))
	}
	return strings.Join(fillStyledLines(lines, height, d.renderSegment(d.styles.Diff, "", "", width, textWidth, false)), "\n")
}

func (d Diff) RenderTextAt(diff string, width, height, yOffset, xOffset int) string {
	width = max(1, width)
	height = max(1, height)
	textWidth := d.TextWidth(width)
	numbered := numberedDiffLines(strings.Split(diff, "\n"))
	if d.viewport.SoftWrap {
		return d.renderWrappedLines(numbered, width, textWidth, height, yOffset)
	}
	return d.renderUnwrappedLines(numbered, width, textWidth, height, yOffset, xOffset)
}

func (d Diff) renderWrappedLines(numbered []numberedDiffLine, width, textWidth, height, yOffset int) string {
	lines := make([]string, 0, height)
	seen := 0
	for _, line := range numbered {
		style := d.lineStyle(line.text)
		highlight := shouldHighlightDiffSyntaxLine(line)
		segments := wrapDisplaySegments(line.text, textWidth)
		for segmentIndex, segment := range segments {
			if seen >= yOffset {
				lines = append(lines, d.renderSegment(style, d.lineNumberGutter(line, segmentIndex > 0), segment, width, textWidth, highlight))
				if len(lines) == height {
					return strings.Join(lines, "\n")
				}
			}
			seen++
		}
	}
	return strings.Join(fillStyledLines(lines, height, d.renderSegment(d.styles.Diff, "", "", width, textWidth, false)), "\n")
}

func (d Diff) renderUnwrappedLines(numbered []numberedDiffLine, width, textWidth, height, yOffset, xOffset int) string {
	lines := make([]string, 0, height)
	for i := yOffset; i < len(numbered) && len(lines) < height; i++ {
		line := numbered[i]
		segment := ansi.Cut(line.text, xOffset, xOffset+textWidth)
		lines = append(lines, d.renderSegment(d.lineStyle(line.text), d.lineNumberGutter(line, false), segment, width, textWidth, shouldHighlightDiffSyntaxLine(line)))
	}
	return strings.Join(fillStyledLines(lines, height, d.renderSegment(d.styles.Diff, "", "", width, textWidth, false)), "\n")
}

func (d Diff) renderSegment(style lipgloss.Style, gutter, segment string, width, textWidth int, highlight bool) string {
	if gutter != "" {
		text := d.renderText(style, segment, textWidth, highlight)
		return gutter + text
	}
	return d.renderText(style, segment, width, highlight)
}

func wrapDisplaySegments(line string, width int) []string {
	width = max(1, width)
	if line == "" {
		return []string{""}
	}
	lineWidth := lipgloss.Width(line)
	segments := make([]string, 0, max(1, (lineWidth+width-1)/width))
	for offset := 0; offset < lineWidth; offset += width {
		segments = append(segments, ansi.Cut(line, offset, offset+width))
	}
	return segments
}

func (d Diff) TextWidth(width int) int {
	return max(1, width-d.GutterWidth())
}

func (d Diff) GutterWidth() int {
	if !d.showLineNumbers {
		return 0
	}
	return 8
}

func fillStyledLines(lines []string, height int, fill string) []string {
	for len(lines) < height {
		lines = append(lines, fill)
	}
	return lines
}

func clampDiff(value, low, high int) int {
	if high < low {
		high = low
	}
	return min(max(value, low), high)
}
