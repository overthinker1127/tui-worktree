package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   []string
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err := f.errs[key]; err != nil {
		return "", err
	}
	return f.outputs[key], nil
}

func TestRepositoryChangesCombinesStatusAndNumstat(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"git status --porcelain=v1 -z":  " M README.md\x00?? scratch.txt\x00",
		"git diff --numstat -z HEAD --": "4\t2\tREADME.md\x00",
	}}
	repo := Repository{Dir: ".", Runner: runner}

	got, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() error = %v", err)
	}

	want := []FileChange{
		{Path: "README.md", Status: Modified, Additions: 4, Deletions: 2},
		{Path: "scratch.txt", Status: Untracked},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Changes() = %#v, want %#v", got, want)
	}
}

func TestRepositoryDiffReturnsUntrackedFileContents(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes", "scratch.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runner := &fakeRunner{outputs: map[string]string{
		"git rev-parse --show-toplevel": root + "\n",
	}}
	repo := Repository{Runner: runner}

	got, err := repo.Diff(context.Background(), FileChange{Path: "notes/scratch.txt", Status: Untracked})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	for _, want := range []string{
		"diff --git a/notes/scratch.txt b/notes/scratch.txt",
		"new file mode 100644",
		"--- /dev/null",
		"+++ b/notes/scratch.txt",
		"+hello",
		"+world",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Diff() = %q, want %q", got, want)
		}
	}
}

func TestRepositoryDiffRequestsUncoloredOutput(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"git rev-parse --show-toplevel":                                         ".",
		"git rev-parse --verify HEAD":                                           "abc123\n",
		"git diff --no-ext-diff --find-renames --color=never HEAD -- README.md": "diff --git a/README.md b/README.md\n",
	}}
	repo := Repository{Runner: runner}

	_, err := repo.Diff(context.Background(), FileChange{Path: "README.md", Status: Modified})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	want := "git diff --no-ext-diff --find-renames --color=never HEAD -- README.md"
	if len(runner.calls) != 3 || runner.calls[2] != want {
		t.Fatalf("Diff() command = %#v, want %q", runner.calls, want)
	}
}

func TestRepositoryChangesWrapsStatusError(t *testing.T) {
	runner := &fakeRunner{
		errs: map[string]error{"git rev-parse --show-toplevel": errors.New("not a repo")},
	}
	repo := Repository{Runner: runner}

	_, err := repo.Changes(context.Background())
	if err == nil || !strings.Contains(err.Error(), "git rev-parse --show-toplevel") {
		t.Fatalf("Changes() error = %v, want git root context", err)
	}
}

func TestRepositoryWorktrees(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"git rev-parse --show-toplevel":                             "/repo\n",
		"git worktree list --porcelain":                             "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\nworktree /repo/.worktrees/feature\nHEAD def456\nbranch refs/heads/feature\n",
		"git symbolic-ref --quiet --short refs/remotes/origin/HEAD": "origin/main\n",
	}}
	repo := Repository{Runner: runner}

	got, err := repo.Worktrees(context.Background())
	if err != nil {
		t.Fatalf("Worktrees() error = %v", err)
	}

	want := []Worktree{
		{Path: "/repo", Branch: "main", Head: "abc123", Current: true, Primary: true, DefaultBranch: true, Protected: true},
		{Path: "/repo/.worktrees/feature", Branch: "feature", Head: "def456"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Worktrees() = %#v, want %#v", got, want)
	}
}

func TestRepositoryDeleteWorktreeRemovesWorktreeThenBranch(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"git rev-parse --show-toplevel": "/repo\n",
	}}
	repo := Repository{Runner: runner}

	err := repo.DeleteWorktree(context.Background(), Worktree{Path: "/repo/.worktrees/feature", Branch: "feature"})
	if err != nil {
		t.Fatalf("DeleteWorktree() error = %v", err)
	}

	want := []string{
		"git rev-parse --show-toplevel",
		"git worktree remove --force /repo/.worktrees/feature",
		"git branch -D feature",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands = %#v, want %#v", runner.calls, want)
	}
}

func TestRepositoryDeleteWorktreeRejectsProtectedBranches(t *testing.T) {
	for _, branch := range []string{"main", "master", "develop", "dev", "release/1.0", "hotfix/login", "production", "staging"} {
		runner := &fakeRunner{}
		repo := Repository{Runner: runner}

		err := repo.DeleteWorktree(context.Background(), Worktree{Path: "/repo/.worktrees/" + branch, Branch: branch})
		if err == nil || !strings.Contains(err.Error(), "protected branch") {
			t.Fatalf("DeleteWorktree(%q) error = %v, want protected branch", branch, err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("DeleteWorktree(%q) commands = %#v, want none", branch, runner.calls)
		}
	}
}
