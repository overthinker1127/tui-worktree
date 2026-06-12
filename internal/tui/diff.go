package tui

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) renderDiffContent(diff string, width int) string {
	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		lines[i] = m.renderDiffSegment(m.diffLineStyle(line), "", line, width, width, shouldHighlightDiffSyntax(line))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDiffViewportContent() string {
	width := m.viewport.Width()
	height := m.viewport.Height()
	if width <= 0 || height <= 0 {
		return ""
	}
	textWidth := m.diffTextWidth(width)
	if m.viewport.SoftWrap {
		return m.renderWrappedDiffViewport(width, textWidth, height)
	}
	return m.renderUnwrappedDiffViewport(width, textWidth, height)
}

func (m Model) renderWrappedDiffViewport(width, textWidth, height int) string {
	lines := make([]string, 0, height)
	offset := m.viewport.YOffset()
	seen := 0
	for _, line := range m.numberedDiffLines() {
		style := m.diffLineStyle(line.text)
		highlight := shouldHighlightDiffSyntax(line.text)
		segments := wrapDisplaySegments(line.text, textWidth)
		for segmentIndex, segment := range segments {
			if seen >= offset {
				lines = append(lines, m.renderDiffSegment(style, m.lineNumberGutter(line, segmentIndex > 0), segment, width, textWidth, highlight))
				if len(lines) == height {
					return strings.Join(lines, "\n")
				}
			}
			seen++
		}
	}
	return strings.Join(fillStyledLines(lines, height, m.renderDiffSegment(m.styles.Diff, "", "", width, textWidth, false)), "\n")
}

func (m Model) renderUnwrappedDiffViewport(width, textWidth, height int) string {
	lines := make([]string, 0, height)
	offset := m.viewport.YOffset()
	xOffset := m.viewport.XOffset()
	numbered := m.numberedDiffLines()
	for i := offset; i < len(numbered) && len(lines) < height; i++ {
		line := numbered[i]
		segment := ansi.Cut(line.text, xOffset, xOffset+textWidth)
		lines = append(lines, m.renderDiffSegment(m.diffLineStyle(line.text), m.lineNumberGutter(line, false), segment, width, textWidth, shouldHighlightDiffSyntax(line.text)))
	}
	return strings.Join(fillStyledLines(lines, height, m.renderDiffSegment(m.styles.Diff, "", "", width, textWidth, false)), "\n")
}

func (m Model) renderDiffSegment(style lipgloss.Style, gutter, segment string, width, textWidth int, highlight bool) string {
	if gutter != "" {
		text := m.renderDiffText(style, segment, textWidth, highlight)
		return gutter + text
	}
	return m.renderDiffText(style, segment, width, highlight)
}

func (m Model) renderDiffText(style lipgloss.Style, segment string, width int, highlight bool) string {
	if !highlight || segment == "" || !containsSyntaxKeyword(segment) {
		return style.Inline(true).Width(width).Render(segment)
	}
	text := m.highlightSyntaxKeywords(segment, style)
	padding := max(0, width-lipgloss.Width(text))
	if padding > 0 {
		text += style.Inline(true).Render(strings.Repeat(" ", padding))
	}
	return text
}

func containsSyntaxKeyword(segment string) bool {
	tokenStart := -1
	for i, r := range segment {
		if isSyntaxIdentRune(r) {
			if tokenStart < 0 {
				tokenStart = i
			}
			continue
		}
		if tokenStart >= 0 {
			if _, ok := syntaxKeywords[segment[tokenStart:i]]; ok {
				return true
			}
			tokenStart = -1
		}
	}
	if tokenStart >= 0 {
		_, ok := syntaxKeywords[segment[tokenStart:]]
		return ok
	}
	return false
}

func (m Model) highlightSyntaxKeywords(segment string, base lipgloss.Style) string {
	var rendered strings.Builder
	last := 0
	tokenStart := -1
	for i, r := range segment {
		if isSyntaxIdentRune(r) {
			if tokenStart < 0 {
				tokenStart = i
			}
			continue
		}
		if tokenStart >= 0 {
			m.writeSyntaxToken(&rendered, base, segment[last:tokenStart], segment[tokenStart:i])
			last = i
			tokenStart = -1
		}
	}
	if tokenStart >= 0 {
		m.writeSyntaxToken(&rendered, base, segment[last:tokenStart], segment[tokenStart:])
		last = len(segment)
	}
	if last < len(segment) {
		rendered.WriteString(base.Inline(true).Render(segment[last:]))
	}
	return rendered.String()
}

func (m Model) writeSyntaxToken(out *strings.Builder, base lipgloss.Style, prefix, token string) {
	if prefix != "" {
		out.WriteString(base.Inline(true).Render(prefix))
	}
	if _, ok := syntaxKeywords[token]; ok {
		out.WriteString(m.styles.DiffKeyword.
			Background(base.GetBackground()).
			Inline(true).
			Render(token))
		return
	}
	out.WriteString(base.Inline(true).Render(token))
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

func (m Model) diffTextWidth(width int) int {
	return max(1, width-m.diffGutterWidth())
}

func (m Model) diffGutterWidth() int {
	if !m.showLineNumbers {
		return 0
	}
	return 8
}

type numberedDiffLine struct {
	text string
	old  int
	new  int
}

func (m Model) numberedDiffLines() []numberedDiffLine {
	lines := make([]numberedDiffLine, 0, len(m.diffLines))
	oldLine, newLine := 0, 0
	seenHunkInFile := false
	for _, line := range m.diffLines {
		if strings.HasPrefix(line, "diff --git") {
			seenHunkInFile = false
			lines = append(lines, numberedDiffLine{text: line})
			continue
		}
		if oldStart, newStart, ok := parseDiffHunkHeader(line); ok {
			if seenHunkInFile {
				lines = append(lines, numberedDiffLine{text: ""})
			}
			oldLine = oldStart
			newLine = newStart
			seenHunkInFile = true
			lines = append(lines, numberedDiffLine{text: line})
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			lines = append(lines, numberedDiffLine{text: line})
		case strings.HasPrefix(line, "+"):
			lines = append(lines, numberedDiffLine{text: line, new: newLine})
			newLine++
		case strings.HasPrefix(line, "-"):
			lines = append(lines, numberedDiffLine{text: line, old: oldLine})
			oldLine++
		case oldLine > 0 || newLine > 0:
			lines = append(lines, numberedDiffLine{text: line, old: oldLine, new: newLine})
			oldLine++
			newLine++
		default:
			lines = append(lines, numberedDiffLine{text: line})
		}
	}
	return lines
}

func parseDiffHunkHeader(line string) (int, int, bool) {
	if !strings.HasPrefix(line, "@@ -") {
		return 0, 0, false
	}
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return 0, 0, false
	}
	oldStart, ok := parseHunkStart(parts[1], '-')
	if !ok {
		return 0, 0, false
	}
	newStart, ok := parseHunkStart(parts[2], '+')
	if !ok {
		return 0, 0, false
	}
	return oldStart, newStart, true
}

func parseHunkStart(value string, prefix byte) (int, bool) {
	if len(value) < 2 || value[0] != prefix {
		return 0, false
	}
	value = value[1:]
	if index := strings.IndexByte(value, ','); index >= 0 {
		value = value[:index]
	}
	var parsed int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		parsed = parsed*10 + int(r-'0')
	}
	if parsed <= 0 {
		parsed = 1
	}
	return parsed, true
}

func (m Model) lineNumberGutter(line numberedDiffLine, continuation bool) string {
	if !m.showLineNumbers {
		return ""
	}
	style := m.styles.Muted.Background(m.styles.Diff.GetBackground())
	if continuation {
		return style.Inline(true).Width(m.diffGutterWidth()).Render("")
	}
	return style.Inline(true).Width(m.diffGutterWidth()).Render(fmt.Sprintf("%5s │ ", lineNumberLabel(line)))
}

func lineNumberLabel(line numberedDiffLine) string {
	if line.new > 0 {
		return fmt.Sprintf("%d", line.new)
	}
	if line.old > 0 {
		return fmt.Sprintf("-%d", line.old)
	}
	return ""
}

func lineNumberText(value int) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func fillStyledLines(lines []string, height int, fill string) []string {
	for len(lines) < height {
		lines = append(lines, fill)
	}
	return lines
}

func (m Model) diffLineStyle(line string) lipgloss.Style {
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff --git"):
		return m.styles.DiffFileHeader
	case strings.HasPrefix(line, "@@"):
		return m.styles.DiffHunk
	case strings.HasPrefix(line, "+"):
		return m.styles.DiffAddition
	case strings.HasPrefix(line, "-"):
		return m.styles.DiffDeletion
	default:
		return m.styles.Diff
	}
}

func shouldHighlightDiffSyntax(line string) bool {
	return line != "" &&
		!strings.HasPrefix(line, "+++") &&
		!strings.HasPrefix(line, "---") &&
		!strings.HasPrefix(line, "diff --git") &&
		!strings.HasPrefix(line, "@@")
}

func isSyntaxIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

var syntaxKeywords = map[string]struct{}{
	"abstract":    {},
	"as":          {},
	"async":       {},
	"await":       {},
	"base":        {},
	"bool":        {},
	"boolean":     {},
	"break":       {},
	"case":        {},
	"catch":       {},
	"chan":        {},
	"class":       {},
	"const":       {},
	"constructor": {},
	"continue":    {},
	"crate":       {},
	"data":        {},
	"defer":       {},
	"def":         {},
	"default":     {},
	"del":         {},
	"delete":      {},
	"do":          {},
	"double":      {},
	"dynamic":     {},
	"elif":        {},
	"else":        {},
	"elseif":      {},
	"enum":        {},
	"event":       {},
	"except":      {},
	"export":      {},
	"extends":     {},
	"extern":      {},
	"fallthrough": {},
	"false":       {},
	"final":       {},
	"finally":     {},
	"float":       {},
	"fn":          {},
	"for":         {},
	"foreach":     {},
	"from":        {},
	"fun":         {},
	"func":        {},
	"function":    {},
	"global":      {},
	"go":          {},
	"goto":        {},
	"guard":       {},
	"if":          {},
	"impl":        {},
	"implements":  {},
	"import":      {},
	"in":          {},
	"inline":      {},
	"interface":   {},
	"internal":    {},
	"is":          {},
	"let":         {},
	"long":        {},
	"match":       {},
	"module":      {},
	"mut":         {},
	"namespace":   {},
	"native":      {},
	"new":         {},
	"nil":         {},
	"nonlocal":    {},
	"null":        {},
	"object":      {},
	"operator":    {},
	"out":         {},
	"override":    {},
	"package":     {},
	"pass":        {},
	"private":     {},
	"property":    {},
	"protected":   {},
	"protocol":    {},
	"pub":         {},
	"public":      {},
	"raise":       {},
	"range":       {},
	"readonly":    {},
	"record":      {},
	"ref":         {},
	"require":     {},
	"return":      {},
	"select":      {},
	"sealed":      {},
	"self":        {},
	"short":       {},
	"signed":      {},
	"sizeof":      {},
	"static":      {},
	"strictfp":    {},
	"struct":      {},
	"super":       {},
	"switch":      {},
	"sync":        {},
	"template":    {},
	"this":        {},
	"throw":       {},
	"throws":      {},
	"trait":       {},
	"true":        {},
	"try":         {},
	"type":        {},
	"typealias":   {},
	"typedef":     {},
	"typename":    {},
	"uint":        {},
	"unchecked":   {},
	"union":       {},
	"unsafe":      {},
	"unsigned":    {},
	"use":         {},
	"using":       {},
	"val":         {},
	"var":         {},
	"virtual":     {},
	"void":        {},
	"volatile":    {},
	"when":        {},
	"where":       {},
	"while":       {},
	"with":        {},
	"yield":       {},
}
