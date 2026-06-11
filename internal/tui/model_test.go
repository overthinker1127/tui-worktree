package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
)

func TestModelViewShowsFileListAndDiff(t *testing.T) {
	tm, err := theme.Preset("tokyonight")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		Theme: theme.NewStyles(tm),
		Changes: []gitview.FileChange{
			{Path: "README.md", Status: gitview.Modified, Additions: 4, Deletions: 2},
		},
		Diff: "diff --git a/README.md b/README.md\n+hello\n-world",
	})

	view := model.View().Content

	for _, want := range []string{"Files changed", "README.md", "+4", "-2", "diff --git"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q in %q", want, view)
		}
	}
}

func TestModelMovesSelectionDown(t *testing.T) {
	tm, err := theme.Preset("dark")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	model := NewModel(Config{
		Theme: theme.NewStyles(tm),
		Changes: []gitview.FileChange{
			{Path: "a.go", Status: gitview.Modified},
			{Path: "b.go", Status: gitview.Added},
		},
	})

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	got := next.(Model).Selected()

	if got.Path != "b.go" {
		t.Fatalf("Selected() = %q, want b.go", got.Path)
	}
}
