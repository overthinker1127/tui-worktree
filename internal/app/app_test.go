package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
)

type fakeRepo struct {
	changes []gitview.FileChange
	diffs   map[string]string
	err     error
	calls   []string
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

func TestParseArgs(t *testing.T) {
	got, err := ParseArgs([]string{"--theme", "kanagawa", "--repo", "/tmp/repo"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if got.Theme != "kanagawa" || got.Dir != "/tmp/repo" {
		t.Fatalf("ParseArgs() = %#v", got)
	}
}

func TestUsageMentionsThemes(t *testing.T) {
	usage := Usage("worktree-diff-tui")
	for _, want := range []string{"tokyonight", "kanagawa", "--theme"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("Usage() missing %q in %q", want, usage)
		}
	}
	if !strings.Contains(usage, "worktree-diff-tui") {
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
