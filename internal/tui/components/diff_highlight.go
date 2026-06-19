package components

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
)

func (d Diff) renderText(style lipgloss.Style, segment string, width int, highlight bool) string {
	if strings.HasPrefix(segment, "@@") {
		return d.renderHunkText(style, segment, width)
	}
	if !highlight || segment == "" || !containsSyntaxKeyword(segment) {
		return style.Inline(true).Width(width).Render(segment)
	}
	text := d.highlightSyntaxKeywords(segment, style)
	padding := max(0, width-lipgloss.Width(text))
	if padding > 0 {
		text += style.Inline(true).Render(strings.Repeat(" ", padding))
	}
	return text
}

func (d Diff) renderHunkText(base lipgloss.Style, segment string, width int) string {
	text := d.highlightHunkRanges(segment, base)
	padding := max(0, width-lipgloss.Width(text))
	if padding > 0 {
		text += base.Inline(true).Render(strings.Repeat(" ", padding))
	}
	return text
}

func (d Diff) highlightHunkRanges(segment string, base lipgloss.Style) string {
	var rendered strings.Builder
	last := 0
	for index := 0; index < len(segment); index++ {
		if segment[index] != '-' && segment[index] != '+' {
			continue
		}
		if index+1 >= len(segment) || segment[index+1] < '0' || segment[index+1] > '9' {
			continue
		}
		end := index + 2
		for end < len(segment) {
			ch := segment[end]
			if ch != ',' && (ch < '0' || ch > '9') {
				break
			}
			end++
		}
		rendered.WriteString(base.Inline(true).Render(segment[last:index]))
		rangeStyle := d.styles.Deleted
		if segment[index] == '+' {
			rangeStyle = d.styles.Added
		}
		rendered.WriteString(rangeStyle.
			Background(base.GetBackground()).
			Inline(true).
			Render(segment[index:end]))
		last = end
		index = end - 1
	}
	if last < len(segment) {
		rendered.WriteString(base.Inline(true).Render(segment[last:]))
	}
	return rendered.String()
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

func (d Diff) highlightSyntaxKeywords(segment string, base lipgloss.Style) string {
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
			d.writeSyntaxToken(&rendered, base, segment[last:tokenStart], segment[tokenStart:i])
			last = i
			tokenStart = -1
		}
	}
	if tokenStart >= 0 {
		d.writeSyntaxToken(&rendered, base, segment[last:tokenStart], segment[tokenStart:])
		last = len(segment)
	}
	if last < len(segment) {
		rendered.WriteString(base.Inline(true).Render(segment[last:]))
	}
	return rendered.String()
}

func (d Diff) writeSyntaxToken(out *strings.Builder, base lipgloss.Style, prefix, token string) {
	if prefix != "" {
		out.WriteString(base.Inline(true).Render(prefix))
	}
	if _, ok := syntaxKeywords[token]; ok {
		out.WriteString(d.styles.DiffKeyword.
			Background(base.GetBackground()).
			Inline(true).
			Render(token))
		return
	}
	out.WriteString(base.Inline(true).Render(token))
}
func (d Diff) lineStyle(line string) lipgloss.Style {
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "diff --git"):
		return d.styles.DiffFileHeader
	case strings.HasPrefix(line, "@@"):
		return d.styles.DiffHunk
	case strings.HasPrefix(line, "+"):
		return d.styles.DiffAddition
	case strings.HasPrefix(line, "-"):
		return d.styles.DiffDeletion
	default:
		return d.styles.Diff
	}
}

func shouldHighlightDiffSyntax(line string) bool {
	return line != "" &&
		!strings.HasPrefix(line, "+++") &&
		!strings.HasPrefix(line, "---") &&
		!strings.HasPrefix(line, "diff --git") &&
		!strings.HasPrefix(line, "@@")
}

func shouldHighlightDiffSyntaxLine(line numberedDiffLine) bool {
	return shouldHighlightDiffSyntax(line.text) && isCodeDiffPath(line.path)
}

func isCodeDiffPath(path string) bool {
	extension := diffPathExtension(path)
	if extension == "" {
		return false
	}
	_, ok := codeDiffExtensions[extension]
	return ok
}

func diffPathExtension(path string) string {
	slash := strings.LastIndexAny(path, `/\`)
	dot := strings.LastIndexByte(path, '.')
	if dot <= slash || dot == len(path)-1 {
		return ""
	}
	return strings.ToLower(path[dot:])
}

var codeDiffExtensions = map[string]struct{}{
	".bash":  {},
	".c":     {},
	".cc":    {},
	".clj":   {},
	".cpp":   {},
	".cs":    {},
	".css":   {},
	".dart":  {},
	".ex":    {},
	".exs":   {},
	".fs":    {},
	".fsx":   {},
	".go":    {},
	".h":     {},
	".hpp":   {},
	".hs":    {},
	".html":  {},
	".java":  {},
	".js":    {},
	".jsx":   {},
	".kt":    {},
	".kts":   {},
	".lua":   {},
	".m":     {},
	".mm":    {},
	".php":   {},
	".pl":    {},
	".pm":    {},
	".ps1":   {},
	".py":    {},
	".r":     {},
	".rb":    {},
	".rs":    {},
	".scala": {},
	".scss":  {},
	".sh":    {},
	".sql":   {},
	".swift": {},
	".ts":    {},
	".tsx":   {},
	".vue":   {},
	".zig":   {},
	".zsh":   {},
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
