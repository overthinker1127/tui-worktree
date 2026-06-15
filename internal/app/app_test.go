package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/tui"
)

type fakeRepo struct {
	changes   []gitview.FileChange
	diffs     map[string]string
	worktrees []gitview.Worktree
	err       error
	calls     []string
	deleted   []gitview.Worktree
}

func (f *fakeRepo) Changes(context.Context) ([]gitview.FileChange, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.changes, nil
}

func (f *fakeRepo) Diff(_ context.Context, change gitview.FileChange) (string, error) {
	f.calls = append(f.calls, change.Path)
	return f.diffs[change.Path], nil
}

func (f *fakeRepo) DeleteWorktree(_ context.Context, worktree gitview.Worktree) error {
	f.deleted = append(f.deleted, worktree)
	return nil
}

func (f *fakeRepo) Worktrees(context.Context) ([]gitview.Worktree, error) {
	if len(f.worktrees) > 0 {
		return f.worktrees, nil
	}
	return []gitview.Worktree{{Path: ".", Branch: "current", Current: true}}, nil
}

func TestParseArgs(t *testing.T) {
	got, err := ParseArgs([]string{"--theme", "kanagawa", "--repo", "/tmp/repo", "--transparent"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if got.Theme != "kanagawa" || got.Dir != "/tmp/repo" || !got.Transparent {
		t.Fatalf("ParseArgs() = %#v", got)
	}
}

func TestParseArgsLeavesThemeEmptyWhenNotProvided(t *testing.T) {
	got, err := ParseArgs(nil)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if got.Theme != "" {
		t.Fatalf("Theme = %q, want empty for config fallback", got.Theme)
	}
}

func TestSaveLoadConfig(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	if err := SaveConfig(UserConfig{Theme: "kanagawa", Transparent: true}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.Theme != "kanagawa" {
		t.Fatalf("Theme = %q, want kanagawa", got.Theme)
	}
	if !got.Transparent {
		t.Fatal("Transparent = false, want true")
	}

	path := filepath.Join(configHome, "tui-worktree", "config.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file missing at %s: %v", path, err)
	}
}

func TestResolveThemeUsesConfigUnlessFlagProvided(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SaveConfig(UserConfig{Theme: "gruvbox-dark"}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if got := ResolveTheme(Options{}); got != "gruvbox-dark" {
		t.Fatalf("ResolveTheme() = %q, want gruvbox-dark", got)
	}
	if got := ResolveTheme(Options{Theme: "kanagawa"}); got != "kanagawa" {
		t.Fatalf("ResolveTheme(flag) = %q, want kanagawa", got)
	}
}

func TestResolveTransparentUsesConfigOrFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := ResolveTransparent(Options{}); got {
		t.Fatal("ResolveTransparent(empty) = true, want false")
	}
	if err := SaveConfig(UserConfig{Transparent: true}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	if got := ResolveTransparent(Options{}); !got {
		t.Fatal("ResolveTransparent(config) = false, want true")
	}
	if got := ResolveTransparent(Options{Transparent: true}); !got {
		t.Fatal("ResolveTransparent(flag) = false, want true")
	}
}

func TestUsageMentionsThemes(t *testing.T) {
	usage := Usage("tui-worktree")
	for _, want := range []string{"tokyonight", "kanagawa", "--theme", "--transparent"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("Usage() missing %q in %q", want, usage)
		}
	}
	if !strings.Contains(usage, "tui-worktree") {
		t.Fatalf("Usage() missing command name: %q", usage)
	}
}

func TestLoadModelRendersRepositoryData(t *testing.T) {
	repo := &fakeRepo{
		changes: []gitview.FileChange{{Path: "main.go", Status: gitview.Modified}},
		diffs:   map[string]string{"main.go": "diff --git a/main.go b/main.go\n+package main"},
	}
	model := LoadModel(context.Background(), repo, "tokyonight")

	view := model.View().Content
	if !strings.Contains(view, "main.go") || !strings.Contains(view, "diff --git") {
		t.Fatalf("LoadModel view = %q", view)
	}
}

func TestLoadModelLoadsOnlySelectedDiffInitially(t *testing.T) {
	repo := &fakeRepo{
		changes: []gitview.FileChange{
			{Path: "a.go", Status: gitview.Modified},
			{Path: "b.go", Status: gitview.Modified},
		},
		diffs: map[string]string{
			"a.go": "diff --git a/a.go b/a.go\n+a",
			"b.go": "diff --git a/b.go b/b.go\n+b",
		},
	}

	_ = LoadModel(context.Background(), repo, "tokyonight")

	if len(repo.calls) != 1 || repo.calls[0] != "a.go" {
		t.Fatalf("Diff calls = %#v, want only selected file", repo.calls)
	}
}

func TestReloadSnapshotDoesNotLoadDiff(t *testing.T) {
	repo := &fakeRepo{
		changes: []gitview.FileChange{
			{Path: "a.go", Status: gitview.Modified, Additions: 1},
		},
		diffs: map[string]string{
			"a.go": "diff --git a/a.go b/a.go\n+a",
		},
	}

	snapshot := loadSnapshot(context.Background(), repo, "", false)

	if len(repo.calls) != 0 {
		t.Fatalf("Diff calls = %#v, want none for reload snapshot", repo.calls)
	}
	if snapshot.Diffs != nil {
		t.Fatalf("reload snapshot Diffs = %#v, want nil", snapshot.Diffs)
	}
	if len(snapshot.Changes) != 1 || snapshot.Changes[0].Path != "a.go" {
		t.Fatalf("reload snapshot Changes = %#v, want a.go", snapshot.Changes)
	}
}

func TestLoadModelRendersGitError(t *testing.T) {
	model := LoadModel(context.Background(), &fakeRepo{err: errors.New("not a git repository")}, "tokyonight")

	view := model.View().Content
	if !strings.Contains(view, "not a git repository") {
		t.Fatalf("LoadModel error view = %q", view)
	}
}

func TestLoadModelKeepsDataWhenThemeIsInvalid(t *testing.T) {
	model := LoadModel(context.Background(), &fakeRepo{
		changes: []gitview.FileChange{{Path: "main.go", Status: gitview.Modified}},
		diffs:   map[string]string{"main.go": "diff --git a/main.go b/main.go\n+package main"},
	}, "not-a-theme")

	view := model.View().Content
	if !strings.Contains(view, "main.go") || !strings.Contains(view, "unknown theme") {
		t.Fatalf("LoadModel invalid theme view = %q", view)
	}
}

func TestLoadModelWiresDeleteWorktree(t *testing.T) {
	repo := &fakeRepo{
		worktrees: []gitview.Worktree{{Path: "/repo/.worktrees/feature", Branch: "feature"}},
	}
	model := LoadModel(context.Background(), repo, "tokyonight")

	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	next, cmd := next.(tui.Model).Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	if cmd == nil {
		t.Fatal("confirm delete returned nil command")
	}
	_, _ = next.(tui.Model).Update(cmd())

	if len(repo.deleted) != 1 || repo.deleted[0].Branch != "feature" {
		t.Fatalf("deleted = %#v, want feature", repo.deleted)
	}
}
