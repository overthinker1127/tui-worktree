package tui

import (
	"strings"

	"github.com/overthinker1127/tui-worktree/internal/tui/components"
)

func editorTargetLine(diff string) int {
	oldLine, newLine := 0, 0
	for _, line := range strings.Split(diff, "\n") {
		if oldStart, newStart, ok := components.ParseDiffHunkHeader(line); ok {
			oldLine = oldStart
			newLine = newStart
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		case strings.HasPrefix(line, "+"):
			return newLine
		case strings.HasPrefix(line, "-"):
			return max(1, newLine)
		case oldLine > 0 || newLine > 0:
			oldLine++
			newLine++
		}
	}
	return 0
}
