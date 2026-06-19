package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const mergeGitCommandTimeout = 2 * time.Minute

func MergeBranch(ctx context.Context, req MergeRequest) error {
	if req.Source.Branch == "" || req.Source.Branch == "detached" {
		return fmt.Errorf("selected worktree has no branch")
	}
	if req.Source.Path == "" {
		return fmt.Errorf("merge source has no worktree path")
	}
	if req.Target.Path == "" {
		return fmt.Errorf("merge target has no worktree path")
	}
	if req.Target.Branch == "" || req.Target.Branch == "detached" {
		return fmt.Errorf("merge target has no branch")
	}
	if err := ensureCleanWorktree(ctx, req.Source.Path, "merge source"); err != nil {
		return err
	}
	if err := ensureCleanWorktree(ctx, req.Target.Path, "merge target"); err != nil {
		return err
	}
	if err := runGit(ctx, req.Target.Path, "switch", req.Target.Branch); err != nil {
		return fmt.Errorf("git switch %s: %w", req.Target.Branch, err)
	}
	if err := runGit(ctx, req.Target.Path, "pull", "--ff-only"); err != nil {
		return fmt.Errorf("git pull --ff-only: %w", err)
	}
	if err := ensureMergeWillApply(ctx, req.Target.Path, req.Source.Branch); err != nil {
		return err
	}
	if err := runGit(ctx, req.Target.Path, "merge", "--no-edit", req.Source.Branch); err != nil {
		return fmt.Errorf("git merge %s: %w", req.Source.Branch, err)
	}
	return nil
}

func ensureCleanWorktree(ctx context.Context, dir string, label string) error {
	out, err := runGitOutput(ctx, dir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("git status %s: %w", label, commandError(err, out))
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("%s has uncommitted or untracked files: %s", label, summarizePorcelainStatus(out))
	}
	return nil
}

func ensureMergeWillApply(ctx context.Context, dir string, sourceBranch string) error {
	out, err := runGitOutput(ctx, dir, "merge-tree", "--write-tree", "--messages", "--name-only", "HEAD", sourceBranch)
	if err == nil {
		return nil
	}
	files := mergeTreeConflictFiles(out)
	if len(files) > 0 {
		return fmt.Errorf("merge would conflict in %s", strings.Join(files, ", "))
	}
	detail := strings.TrimSpace(out)
	if strings.Contains(detail, "CONFLICT") {
		return fmt.Errorf("merge would conflict: %s", detail)
	}
	return fmt.Errorf("git merge-tree %s: %w", sourceBranch, commandError(err, out))
}

func runGit(ctx context.Context, dir string, args ...string) error {
	out, err := runGitOutput(ctx, dir, args...)
	if err != nil {
		return commandError(err, out)
	}
	return nil
}

func runGitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, mergeGitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil && cmdCtx.Err() != nil {
		return string(out), fmt.Errorf("%w after %s", cmdCtx.Err(), mergeGitCommandTimeout)
	}
	return string(out), err
}

func commandError(err error, out string) error {
	if err != nil {
		detail := strings.TrimSpace(out)
		if detail != "" {
			return fmt.Errorf("%w: %s", err, detail)
		}
		return err
	}
	return nil
}

func summarizePorcelainStatus(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	files := make([]string, 0, min(len(lines), 5))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 2 {
			line = strings.TrimSpace(line[2:])
		}
		if line == "" {
			continue
		}
		files = append(files, line)
		if len(files) == 5 {
			break
		}
	}
	if len(files) == 0 {
		return strings.TrimSpace(out)
	}
	if len(lines) > len(files) {
		return fmt.Sprintf("%s, and %d more", strings.Join(files, ", "), len(lines)-len(files))
	}
	return strings.Join(files, ", ")
}

func mergeTreeConflictFiles(out string) []string {
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(files) > 0 {
				break
			}
			continue
		}
		if isObjectID(line) {
			continue
		}
		if strings.HasPrefix(line, "Auto-merging ") || strings.HasPrefix(line, "CONFLICT") {
			break
		}
		files = append(files, line)
	}
	return files
}

func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
