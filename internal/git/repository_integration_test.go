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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
