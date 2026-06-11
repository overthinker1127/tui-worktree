package tui

import (
	"context"
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
	for _, want := range []string{"󰈙", ""} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing Nerd Font symbol %q in %q", want, view)
		}
	}
}

func TestModelMovesSelectionDown(t *testing.T) {
	tm, err := theme.Preset("tokyonight")
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

func TestQuestionMarkTogglesHelp(t *testing.T) {
	model := testModel(t)

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	view := next.(Model).View().Content

	for _, want := range []string{"Help", " r refresh", " t themes"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help view missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "mouse select") {
		t.Fatalf("help view should not explain mouse selection: %q", view)
	}
}

func TestThemePickerAppliesTheme(t *testing.T) {
	model := testModel(t)
	model.themeNames = []string{"tokyonight", "gruvbox"}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next, _ = next.(Model).Update(tea.KeyPressMsg(tea.Key{Code: '\r'}))
	got := next.(Model)

	if got.themeName != "gruvbox" {
		t.Fatalf("themeName = %q, want gruvbox", got.themeName)
	}
}

func TestRefreshReloadsChanges(t *testing.T) {
	model := testModel(t)
	model.reload = func(context.Context) Snapshot {
		return Snapshot{
			Changes: []gitview.FileChange{{Path: "fresh.go", Status: gitview.Added}},
			Diffs:   map[string]string{"fresh.go": "diff --git a/fresh.go b/fresh.go\n+fresh"},
		}
	}

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	if cmd == nil {
		t.Fatal("refresh command is nil")
	}
	msg := cmd()
	next, _ = next.(Model).Update(msg)
	got := next.(Model)

	if got.Selected().Path != "fresh.go" {
		t.Fatalf("Selected() = %q, want fresh.go", got.Selected().Path)
	}
	if !strings.Contains(got.View().Content, "fresh") {
		t.Fatalf("View() missing refreshed diff: %q", got.View().Content)
	}
}

func TestRefreshUsesConfiguredContext(t *testing.T) {
	model := testModel(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	model.context = ctx
	model.reload = func(ctx context.Context) Snapshot {
		if ctx.Err() == nil {
			t.Fatal("reload context was not canceled")
		}
		return Snapshot{}
	}

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	if cmd == nil {
		t.Fatal("refresh command is nil")
	}
	_ = cmd()
}

func TestMouseClickSelectsFile(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Added},
	}
	model.refreshDiff()

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: 2, Y: 4}))
	got := next.(Model).Selected()

	if got.Path != "b.go" {
		t.Fatalf("Selected() = %q, want b.go", got.Path)
	}
}

func TestMouseClickSelectsTheme(t *testing.T) {
	model := testModel(t)
	model.themeNames = []string{"tokyonight", "gruvbox"}
	model.openThemePicker()

	next, _ := model.Update(tea.MouseClickMsg(tea.Mouse{X: 2, Y: 4}))
	got := next.(Model)

	if got.themeName != "gruvbox" {
		t.Fatalf("themeName = %q, want gruvbox", got.themeName)
	}
	if got.pickingTheme {
		t.Fatal("theme picker still open after mouse selection")
	}
}

func testModel(t *testing.T) Model {
	t.Helper()
	tm, err := theme.Preset("tokyonight")
	if err != nil {
		t.Fatalf("Preset() error = %v", err)
	}
	return NewModel(Config{
		ThemeName: "tokyonight",
		Theme:     theme.NewStyles(tm),
		Changes:   []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}},
		Diffs:     map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"},
	})
}
