package app

import (
	"context"
	"errors"
	"os"
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

type fakeRootRepo struct {
	err error
}

func (f fakeRootRepo) Root(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "/repo", nil
}

func TestParseArgs(t *testing.T) {
	got, err := ParseArgs([]string{"--theme", "kanagawa-wave", "--repo", "/tmp/repo", "--transparent"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if got.Theme != "kanagawa-wave" || got.Dir != "/tmp/repo" || !got.Transparent {
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

func TestParseArgsVersion(t *testing.T) {
	got, err := ParseArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if !got.Version {
		t.Fatal("Version = false, want true")
	}
}

func TestSaveLoadConfig(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	if err := SaveConfig(UserConfig{Theme: "kanagawa-wave", Transparent: true}); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got.Theme != "kanagawa-wave" {
		t.Fatalf("Theme = %q, want kanagawa-wave", got.Theme)
	}
	if !got.Transparent {
		t.Fatal("Transparent = false, want true")
	}

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}
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
	if got := ResolveTheme(Options{Theme: "kanagawa-wave"}); got != "kanagawa-wave" {
		t.Fatalf("ResolveTheme(flag) = %q, want kanagawa-wave", got)
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
	for _, want := range []string{"tokyonight", "kanagawa-wave", "--theme", "--transparent", "--version"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("Usage() missing %q in %q", want, usage)
		}
	}
	if strings.Contains(usage, "kanagawa,") {
		t.Fatalf("Usage() should not include kanagawa alias: %q", usage)
	}
	if !strings.Contains(usage, "tui-worktree") {
		t.Fatalf("Usage() missing command name: %q", usage)
	}
}

func TestVersionString(t *testing.T) {
	old := BuildVersion
	BuildVersion = "1.2.3"
	t.Cleanup(func() {
		BuildVersion = old
	})

	if got, want := Version("tui-worktree"), "tui-worktree 1.2.3\n"; got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
}

func TestEnsureRepositoryReturnsCliMessageWhenRepositoryMissing(t *testing.T) {
	err := ensureRepository(context.Background(), fakeRootRepo{err: errors.New("fatal: not a git repository")}, "/tmp/no-repo")
	if err == nil {
		t.Fatal("ensureRepository() error = nil, want missing repository error")
	}
	if got, want := err.Error(), "not a git repository: /tmp/no-repo"; got != want {
		t.Fatalf("ensureRepository() error = %q, want %q", got, want)
	}
}

func TestEnsureRepositoryAcceptsRepository(t *testing.T) {
	if err := ensureRepository(context.Background(), fakeRootRepo{}, "/repo"); err != nil {
		t.Fatalf("ensureRepository() error = %v", err)
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
	runCommand(cmd)

	if len(repo.deleted) != 1 || repo.deleted[0].Branch != "feature" {
		t.Fatalf("deleted = %#v, want feature", repo.deleted)
	}
}

func runCommand(cmd tea.Cmd) {
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, batchedCmd := range batch {
			if batchedCmd != nil {
				runCommand(batchedCmd)
			}
		}
	}
}
