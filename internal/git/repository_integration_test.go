package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryWithRealGitWorktree(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")

	if err := os.WriteFile(readme, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("modify README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scratch.txt"), []byte("scratch\n"), 0o644); err != nil {
		t.Fatalf("write scratch: %v", err)
	}

	repo := Repository{Dir: dir}
	changes, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() error = %v", err)
	}

	if len(changes) != 2 {
		t.Fatalf("Changes() len = %d, want 2: %#v", len(changes), changes)
	}
	if changes[0].Path != "README.md" || changes[0].Additions != 1 {
		t.Fatalf("README change = %#v, want one addition", changes[0])
	}
	if changes[1].Path != "scratch.txt" || changes[1].Status != Untracked {
		t.Fatalf("scratch change = %#v, want untracked", changes[1])
	}
	for _, change := range changes {
		if change.Fingerprint == "" {
			t.Fatalf("change %s fingerprint is empty: %#v", change.Path, change)
		}
	}
}

func TestRepositoryChangesFingerprintChangesWhenContentChanges(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte("same\nold\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")

	if err := os.WriteFile(path, []byte("same\none\n"), 0o644); err != nil {
		t.Fatalf("modify README first: %v", err)
	}
	repo := Repository{Dir: dir}
	first, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() first error = %v", err)
	}
	if len(first) != 1 || first[0].Fingerprint == "" {
		t.Fatalf("first Changes() = %#v, want fingerprint", first)
	}

	if err := os.WriteFile(path, []byte("same\ntwo\n"), 0o644); err != nil {
		t.Fatalf("modify README second: %v", err)
	}
	second, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() second error = %v", err)
	}
	if len(second) != 1 || second[0].Fingerprint == "" {
		t.Fatalf("second Changes() = %#v, want fingerprint", second)
	}
	if first[0].Additions != second[0].Additions || first[0].Deletions != second[0].Deletions {
		t.Fatalf("line stats changed unexpectedly: first=%#v second=%#v", first[0], second[0])
	}
	if first[0].Fingerprint == second[0].Fingerprint {
		t.Fatalf("fingerprint did not change: first=%#v second=%#v", first[0], second[0])
	}
}

func TestRepositoryHandlesSpacesAndPureRename(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	spaced := filepath.Join(dir, "a b.txt")
	oldPath := filepath.Join(dir, "old name.txt")
	if err := os.WriteFile(spaced, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write spaced file: %v", err)
	}
	if err := os.WriteFile(oldPath, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("write old file: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	if err := os.WriteFile(spaced, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("modify spaced file: %v", err)
	}
	runGit(t, dir, "mv", "old name.txt", "new name.txt")

	repo := Repository{Dir: dir}
	changes, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() error = %v", err)
	}

	var spacedChange, renameChange FileChange
	for _, change := range changes {
		switch change.Path {
		case "a b.txt":
			spacedChange = change
		case "new name.txt":
			renameChange = change
		}
	}
	if spacedChange.Path != "a b.txt" || spacedChange.Additions != 1 {
		t.Fatalf("spaced change = %#v, want path with one addition", spacedChange)
	}
	if renameChange.Status != Renamed || renameChange.OldPath != "old name.txt" {
		t.Fatalf("rename change = %#v, want old/new rename", renameChange)
	}

	diff, err := repo.Diff(context.Background(), renameChange)
	if err != nil {
		t.Fatalf("Diff(rename) error = %v", err)
	}
	if !strings.Contains(diff, "rename from old name.txt") || !strings.Contains(diff, "rename to new name.txt") {
		t.Fatalf("rename diff = %q, want rename metadata", diff)
	}
}

func TestRepositoryDiffWorksFromSubdirectory(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("modify root file: %v", err)
	}

	repo := Repository{Dir: filepath.Join(dir, "sub")}
	changes, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() error = %v", err)
	}
	if len(changes) != 1 || changes[0].Path != "root.txt" {
		t.Fatalf("Changes() = %#v, want root.txt from subdir", changes)
	}
	diff, err := repo.Diff(context.Background(), changes[0])
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if !strings.Contains(diff, "+world") {
		t.Fatalf("Diff() = %q, want root diff from subdir", diff)
	}
}

func TestRepositoryListsLinkedWorktrees(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	linked := filepath.Join(t.TempDir(), "feature")
	runGit(t, dir, "worktree", "add", "-b", "feature", linked)

	repo := Repository{Dir: dir}
	worktrees, err := repo.Worktrees(context.Background())
	if err != nil {
		t.Fatalf("Worktrees() error = %v", err)
	}

	branches := map[string]bool{}
	for _, worktree := range worktrees {
		branches[worktree.Branch] = true
	}
	if len(worktrees) != 2 || !branches["main"] || !branches["feature"] {
		t.Fatalf("Worktrees() = %#v, want main and feature", worktrees)
	}
}

func TestRepositoryChangesInUnbornRepo(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "scratch.txt"), []byte("scratch\n"), 0o644); err != nil {
		t.Fatalf("write scratch: %v", err)
	}

	repo := Repository{Dir: dir}
	changes, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() error = %v", err)
	}
	if len(changes) != 1 || changes[0].Path != "scratch.txt" || changes[0].Status != Untracked {
		t.Fatalf("Changes() = %#v, want untracked file in unborn repo", changes)
	}
}

func TestRepositoryDiffInUnbornRepoUsesCachedDiff(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "added.txt"), []byte("added\n"), 0o644); err != nil {
		t.Fatalf("write added: %v", err)
	}
	runGit(t, dir, "add", "added.txt")

	repo := Repository{Dir: dir}
	changes, err := repo.Changes(context.Background())
	if err != nil {
		t.Fatalf("Changes() error = %v", err)
	}
	if len(changes) != 1 || changes[0].Status != Added {
		t.Fatalf("Changes() = %#v, want staged added file", changes)
	}

	diff, err := repo.Diff(context.Background(), changes[0])
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if !strings.Contains(diff, "new file mode") || !strings.Contains(diff, "+added") {
		t.Fatalf("Diff() = %q, want cached new-file diff", diff)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
