package command

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const editorDefaultScript = `${EDITOR:-vi} "$1"`

func OpenEditorCommand(editor, worktreeDir, filePath string, line int) *exec.Cmd {
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
	}
	if worktreeDir == "" {
		worktreeDir = "."
	}
	cmd := exec.Command("sh", "-c", editorLaunchScript(editor, line), "editor", filePath)
	cmd.Dir = worktreeDir
	cmd.Env = append(os.Environ(), "EDITOR="+editor, fmt.Sprintf("LINE=%d", line))
	return cmd
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
