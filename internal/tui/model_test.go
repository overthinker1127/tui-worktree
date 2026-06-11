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

func TestMouseClickLoadsMissingDiffLazily(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Modified},
	}
	model.diffs = map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"}
	model.loadDiff = func(_ context.Context, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n+b"
	}
	model.refreshDiff()

	next, cmd := model.Update(tea.MouseClickMsg(tea.Mouse{X: 2, Y: 4}))
	if cmd == nil {
		t.Fatal("mouse selection did not request lazy diff load")
	}
	msg := cmd()
	next, _ = next.(Model).Update(msg)
	got := next.(Model)

	if !strings.Contains(got.View().Content, "+b") {
		t.Fatalf("View() missing lazily loaded mouse diff: %q", got.View().Content)
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

func TestFileListWindowsLargeChangeSets(t *testing.T) {
	model := testModel(t)
	model.width = 90
	model.height = 9
	model.changes = make([]gitview.FileChange, 20)
	model.diffs = map[string]string{}
	for i := range model.changes {
		model.changes[i] = gitview.FileChange{Path: "file-" + string(rune('a'+i)) + ".go", Status: gitview.Modified}
	}
	model.refreshDiff()

	firstView := model.View().Content
	if strings.Contains(firstView, "file-t.go") {
		t.Fatalf("initial view included offscreen file: %q", firstView)
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "G", Code: 'G'}))
	lastView := next.(Model).View().Content
	if !strings.Contains(lastView, "file-t.go") || strings.Contains(lastView, "file-a.go") {
		t.Fatalf("last view did not window around selected file: %q", lastView)
	}
}

func TestSelectionLoadsMissingDiffLazily(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{
		{Path: "a.go", Status: gitview.Modified},
		{Path: "b.go", Status: gitview.Modified},
	}
	model.diffs = map[string]string{"a.go": "diff --git a/a.go b/a.go\n+a"}
	model.loadDiff = func(_ context.Context, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n+b"
	}
	model.refreshDiff()

	next, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	if cmd == nil {
		t.Fatal("selection change did not request lazy diff load")
	}
	msg := cmd()
	next, _ = next.(Model).Update(msg)
	got := next.(Model)

	if !strings.Contains(got.View().Content, "+b") {
		t.Fatalf("View() missing lazily loaded diff: %q", got.View().Content)
	}
}

func TestStaleLazyDiffDoesNotOverwriteRefreshedSnapshot(t *testing.T) {
	model := testModel(t)
	model.changes = []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}}
	model.diffs = map[string]string{}
	model.loadDiff = func(_ context.Context, change gitview.FileChange) string {
		return "diff --git a/" + change.Path + " b/" + change.Path + "\n+stale"
	}
	model.refreshDiff()

	cmd := model.ensureSelectedDiffCmd()
	if cmd == nil {
		t.Fatal("expected lazy diff command")
	}
	model.applySnapshot(Snapshot{
		Changes: []gitview.FileChange{{Path: "a.go", Status: gitview.Modified}},
		Diffs:   map[string]string{"a.go": "diff --git a/a.go b/a.go\n+fresh"},
	})

	next, _ := model.Update(cmd())
	got := next.(Model)
	view := got.View().Content
	if strings.Contains(view, "stale") || !strings.Contains(view, "fresh") {
		t.Fatalf("stale lazy diff overwrote refreshed snapshot: %q", view)
	}
}

func TestStaleRefreshDoesNotOverwriteNewerSnapshot(t *testing.T) {
	model := testModel(t)
	model.refreshGeneration = 2
	model.applySnapshot(Snapshot{
		Changes: []gitview.FileChange{{Path: "fresh.go", Status: gitview.Modified}},
		Diffs:   map[string]string{"fresh.go": "diff --git a/fresh.go b/fresh.go\n+fresh"},
	})
	model.refreshGeneration = 2

	next, _ := model.Update(reloadMsg{
		generation: 1,
		snapshot: Snapshot{
			Changes: []gitview.FileChange{{Path: "stale.go", Status: gitview.Modified}},
			Diffs:   map[string]string{"stale.go": "diff --git a/stale.go b/stale.go\n+stale"},
		},
	})
	got := next.(Model)
	view := got.View().Content
	if strings.Contains(view, "stale.go") || !strings.Contains(view, "fresh.go") {
		t.Fatalf("stale refresh overwrote newer snapshot: %q", view)
	}
}

func TestRefreshStartInvalidatesPendingLazyDiff(t *testing.T) {
	model := testModel(t)
	model.diffs = map[string]string{}
	model.loadDiff = func(context.Context, gitview.FileChange) string {
		return "diff --git a/a.go b/a.go\n+stale"
	}
	model.refreshDiff()
	cmd := model.ensureSelectedDiffCmd()
	if cmd == nil {
		t.Fatal("expected lazy diff command")
	}

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	next, _ = next.(Model).Update(cmd())
	got := next.(Model)

	if strings.Contains(got.View().Content, "stale") {
		t.Fatalf("pending lazy diff survived refresh start: %q", got.View().Content)
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
