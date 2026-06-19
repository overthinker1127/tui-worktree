package components

import (
	"fmt"
	"strings"
)

type numberedDiffLine struct {
	text string
	old  int
	new  int
	path string
}

func (d Diff) numberedLines() []numberedDiffLine {
	return numberedDiffLines(d.lines)
}

func numberedDiffLines(diffLines []string) []numberedDiffLine {
	lines := make([]numberedDiffLine, 0, len(diffLines))
	oldLine, newLine := 0, 0
	seenHunkInFile := false
	currentPath := ""
	for _, line := range diffLines {
		if strings.HasPrefix(line, "diff --git") {
			currentPath = parseDiffGitPath(line)
			seenHunkInFile = false
			lines = append(lines, numberedDiffLine{text: line, path: currentPath})
			continue
		}
		if path, ok := parseDiffFilePath(line, "+++ b/"); ok {
			currentPath = path
		} else if currentPath == "" {
			if path, ok := parseDiffFilePath(line, "--- a/"); ok {
				currentPath = path
			}
		}
		if oldStart, newStart, ok := parseDiffHunkHeader(line); ok {
			if seenHunkInFile {
				lines = append(lines, numberedDiffLine{text: "", path: currentPath})
			}
			oldLine = oldStart
			newLine = newStart
			seenHunkInFile = true
			lines = append(lines, numberedDiffLine{text: line, path: currentPath})
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			lines = append(lines, numberedDiffLine{text: line, path: currentPath})
		case strings.HasPrefix(line, "+"):
			lines = append(lines, numberedDiffLine{text: line, new: newLine, path: currentPath})
			newLine++
		case strings.HasPrefix(line, "-"):
			lines = append(lines, numberedDiffLine{text: line, old: oldLine, path: currentPath})
			oldLine++
		case oldLine > 0 || newLine > 0:
			lines = append(lines, numberedDiffLine{text: line, old: oldLine, new: newLine, path: currentPath})
			oldLine++
			newLine++
		default:
			lines = append(lines, numberedDiffLine{text: line, path: currentPath})
		}
	}
	return lines
}

func ParseDiffHunkHeader(line string) (int, int, bool) {
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

func parseDiffHunkHeader(line string) (int, int, bool) {
	return ParseDiffHunkHeader(line)
}

func parseDiffGitPath(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return ""
	}
	if path := trimDiffPathPrefix(parts[3], "b/"); path != "" {
		return path
	}
	return trimDiffPathPrefix(parts[2], "a/")
}

func parseDiffFilePath(line, prefix string) (string, bool) {
	path, ok := strings.CutPrefix(line, prefix)
	if !ok || path == "/dev/null" {
		return "", false
	}
	return strings.TrimSpace(path), true
}

func trimDiffPathPrefix(path, prefix string) string {
	path = strings.Trim(path, `"`)
	path, _ = strings.CutPrefix(path, prefix)
	if path == "/dev/null" {
		return ""
	}
	return path
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

func (d Diff) lineNumberGutter(line numberedDiffLine, continuation bool) string {
	if !d.showLineNumbers {
		return ""
	}
	style := d.styles.Muted.Background(d.styles.Diff.GetBackground())
	if continuation {
		return style.Inline(true).Width(d.GutterWidth()).Render("")
	}
	return style.Inline(true).Width(d.GutterWidth()).Render(fmt.Sprintf("%5s │ ", lineNumberLabel(line)))
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
