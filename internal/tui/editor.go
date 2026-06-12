package tui

import (
	"path/filepath"
	"strings"
)

const editorDefaultScript = `${EDITOR:-vi} "$1"`

func editorTargetLine(diff string) int {
	oldLine, newLine := 0, 0
	for _, line := range strings.Split(diff, "\n") {
		if oldStart, newStart, ok := parseDiffHunkHeader(line); ok {
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

func editorLaunchScript(editor string, line int) string {
	if line <= 0 {
		return editorDefaultScript
	}
	switch editorKind(editor) {
	case "plus-line":
		return `${EDITOR:-vi} "+${LINE}" "$1"`
	case "goto":
		return `${EDITOR:-vi} --goto "$1:$LINE"`
	case "path-line":
		return `${EDITOR:-vi} "$1:$LINE"`
	case "line-flag":
		return `${EDITOR:-vi} --line "$LINE" "$1"`
	default:
		return editorDefaultScript
	}
}

func editorKind(editor string) string {
	name := editorName(editor)
	switch name {
	case "vi", "view", "vim", "nvim", "gvim", "mvim", "nano", "micro", "emacs", "emacsclient", "kak":
		return "plus-line"
	case "code", "code-insiders", "codium", "vscodium", "cursor", "windsurf":
		return "goto"
	case "subl", "sublime_text", "zed", "hx", "helix":
		return "path-line"
	case "idea", "goland", "webstorm", "phpstorm", "pycharm", "rubymine", "clion", "rider":
		return "line-flag"
	default:
		return ""
	}
}

func editorName(editor string) string {
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return "vi"
	}
	name := strings.Trim(fields[0], `"'`)
	return filepath.Base(name)
}
