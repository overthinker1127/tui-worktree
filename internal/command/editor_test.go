package command

import (
	"slices"
	"testing"
)

func TestEditorLaunchScriptSupportsLineAwareEditors(t *testing.T) {
	for _, tc := range []struct {
		editor string
		line   int
		want   string
	}{
		{editor: "nvim", line: 12, want: `${EDITOR:-vi} "+${LINE}" "$1"`},
		{editor: "vim -f", line: 12, want: `${EDITOR:-vi} "+${LINE}" "$1"`},
		{editor: "code --wait", line: 12, want: `${EDITOR:-vi} --goto "$1:$LINE"`},
		{editor: "code-insiders", line: 12, want: `${EDITOR:-vi} --goto "$1:$LINE"`},
		{editor: "true", line: 12, want: `${EDITOR:-vi} "$1"`},
		{editor: "nvim", line: 0, want: `${EDITOR:-vi} "$1"`},
	} {
		t.Run(tc.editor, func(t *testing.T) {
			if got := editorLaunchScript(tc.editor, tc.line); got != tc.want {
				t.Fatalf("editorLaunchScript(%q, %d) = %q, want %q", tc.editor, tc.line, got, tc.want)
			}
		})
	}
}

func TestOpenEditorCommandUsesEditorEnvWhenEditorIsEmpty(t *testing.T) {
	t.Setenv("EDITOR", "code --wait")

	cmd := OpenEditorCommand("", "/repo", "main.go", 12)

	if cmd.Dir != "/repo" {
		t.Fatalf("cmd.Dir = %q, want /repo", cmd.Dir)
	}
	if !slices.Contains(cmd.Env, "EDITOR=code --wait") {
		t.Fatalf("cmd.Env should preserve EDITOR from environment: %#v", cmd.Env)
	}
	if !slices.Contains(cmd.Env, "LINE=12") {
		t.Fatalf("cmd.Env should include target line: %#v", cmd.Env)
	}
}

func TestOpenEditorCommandFallsBackToViWhenEditorIsUnset(t *testing.T) {
	t.Setenv("EDITOR", "")

	cmd := OpenEditorCommand("", "", "main.go", 0)

	if cmd.Dir != "." {
		t.Fatalf("cmd.Dir = %q, want .", cmd.Dir)
	}
	if !slices.Contains(cmd.Env, "EDITOR=vi") {
		t.Fatalf("cmd.Env should fall back to vi: %#v", cmd.Env)
	}
}
