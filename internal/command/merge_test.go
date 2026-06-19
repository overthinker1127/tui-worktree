package command

import (
	"context"
	"os"
	"strings"
	"testing"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
)

func TestMergeBranchSwitchesPullsAndMerges(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()
	logPath := installFakeGit(t, "")

	err := MergeBranch(context.Background(), MergeRequest{
		Source: gitview.Worktree{Path: sourceDir, Branch: "feature"},
		Target: gitview.Worktree{Path: dir, Branch: "main"},
	})
	if err != nil {
		t.Fatalf("MergeBranch() error = %v", err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git args: %v", err)
	}

	want := strings.Join([]string{
		"status --porcelain --untracked-files=all",
		"status --porcelain --untracked-files=all",
		"switch main",
		"pull --ff-only",
		"merge-tree --write-tree --messages --name-only HEAD feature",
		"merge --no-edit feature",
	}, "\n")
	if strings.TrimSpace(string(got)) != want {
		t.Fatalf("git args = %q, want %q", strings.TrimSpace(string(got)), want)
	}
}

func TestMergeBranchBlocksDirtySourceWorktree(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	logPath := installFakeGit(t, "if [ \"$PWD\" = "+sourceDir+" ]; then printf '?? scratch.txt\\n'; fi\n")

	err := MergeBranch(context.Background(), MergeRequest{
		Source: gitview.Worktree{Path: sourceDir, Branch: "feature"},
		Target: gitview.Worktree{Path: targetDir, Branch: "main"},
	})
	if err == nil {
		t.Fatal("MergeBranch() error = nil, want dirty source error")
	}
	if got := err.Error(); !strings.Contains(got, "merge source") || !strings.Contains(got, "scratch.txt") {
		t.Fatalf("error = %q, want dirty source file", got)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git args: %v", err)
	}
	if strings.TrimSpace(string(got)) != "status --porcelain --untracked-files=all" {
		t.Fatalf("git args = %q, want source status only", strings.TrimSpace(string(got)))
	}
}

func TestMergeBranchBlocksDirtyTargetWorktree(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	logPath := installFakeGit(t, "if [ \"$PWD\" = "+targetDir+" ]; then printf ' M README.md\\n?? scratch.txt\\n'; fi\n")

	err := MergeBranch(context.Background(), MergeRequest{
		Source: gitview.Worktree{Path: sourceDir, Branch: "feature"},
		Target: gitview.Worktree{Path: targetDir, Branch: "main"},
	})
	if err == nil {
		t.Fatal("MergeBranch() error = nil, want dirty target error")
	}
	if got := err.Error(); !strings.Contains(got, "merge target") || !strings.Contains(got, "README.md") || !strings.Contains(got, "scratch.txt") {
		t.Fatalf("error = %q, want dirty target files", got)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git args: %v", err)
	}
	want := strings.Join([]string{
		"status --porcelain --untracked-files=all",
		"status --porcelain --untracked-files=all",
	}, "\n")
	if strings.TrimSpace(string(got)) != want {
		t.Fatalf("git args = %q, want status checks only", strings.TrimSpace(string(got)))
	}
}

func TestMergeBranchBlocksConflictingMerge(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	logPath := installFakeGit(t, `if [ "$*" = "merge-tree --write-tree --messages --name-only HEAD feature" ]; then
printf '0123456789012345678901234567890123456789\nREADME.md\ninternal/tui/model.go\n\nAuto-merging README.md\nCONFLICT (content): Merge conflict in README.md\n'
exit 1
fi
`)

	err := MergeBranch(context.Background(), MergeRequest{
		Source: gitview.Worktree{Path: sourceDir, Branch: "feature"},
		Target: gitview.Worktree{Path: targetDir, Branch: "main"},
	})
	if err == nil {
		t.Fatal("MergeBranch() error = nil, want conflict error")
	}
	if got := err.Error(); !strings.Contains(got, "merge would conflict") || !strings.Contains(got, "README.md") || !strings.Contains(got, "internal/tui/model.go") {
		t.Fatalf("error = %q, want conflict files", got)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git args: %v", err)
	}
	want := strings.Join([]string{
		"status --porcelain --untracked-files=all",
		"status --porcelain --untracked-files=all",
		"switch main",
		"pull --ff-only",
		"merge-tree --write-tree --messages --name-only HEAD feature",
	}, "\n")
	if strings.TrimSpace(string(got)) != want {
		t.Fatalf("git args = %q, want preflight without merge", strings.TrimSpace(string(got)))
	}
}

func installFakeGit(t *testing.T, body string) string {
	t.Helper()
	bin := t.TempDir()
	logPath := bin + "/git-args"
	gitPath := bin + "/git"
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\n" + body
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}
