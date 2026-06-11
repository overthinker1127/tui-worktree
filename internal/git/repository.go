package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %v: %w: %s", name, args, err, string(out))
	}
	return string(out), nil
}

type Repository struct {
	Dir    string
	Runner Runner
}

func (r Repository) Changes(ctx context.Context) ([]FileChange, error) {
	runner := r.runner()
	dir, err := r.root(ctx)
	if err != nil {
		return nil, err
	}

	statusOut, err := runner.Run(ctx, dir, "git", "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	changes, err := ParsePorcelainStatus(statusOut)
	if err != nil {
		return nil, err
	}

	if !r.hasHead(ctx, dir) {
		return changes, nil
	}

	numstatOut, err := runner.Run(ctx, dir, "git", "diff", "--numstat", "-z", "HEAD", "--")
	if err != nil {
		return nil, fmt.Errorf("git diff --numstat: %w", err)
	}
	stats, err := ParseNumstat(numstatOut)
	if err != nil {
		return nil, err
	}
	return ApplyLineStats(changes, stats), nil
}

func (r Repository) Diff(ctx context.Context, change FileChange) (string, error) {
	if change.Status == Untracked {
		return fmt.Sprintf("Untracked file: %s\n\nNo diff is available until the file is added to git.", change.Path), nil
	}
	dir, err := r.root(ctx)
	if err != nil {
		return "", err
	}
	args := []string{"diff", "--no-ext-diff", "--find-renames", "--color=never"}
	if r.hasHead(ctx, dir) {
		args = append(args, "HEAD")
	} else {
		args = append(args, "--cached")
	}
	args = append(args, "--")
	if change.Status == Renamed && change.OldPath != "" {
		args = append(args, change.OldPath, change.Path)
	} else {
		args = append(args, change.Path)
	}
	out, err := r.runner().Run(ctx, dir, "git", args...)
	if err != nil {
		return "", fmt.Errorf("git diff %s: %w", change.Path, err)
	}
	if out == "" {
		return fmt.Sprintf("No diff for %s", change.Path), nil
	}
	return out, nil
}

func (r Repository) Root(ctx context.Context) (string, error) {
	return r.root(ctx)
}

func (r Repository) Worktrees(ctx context.Context) ([]Worktree, error) {
	root, err := r.root(ctx)
	if err != nil {
		return nil, err
	}
	out, err := r.runner().Run(ctx, root, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	worktrees, err := ParseWorktreeList(out, root)
	if err != nil {
		return nil, err
	}
	defaultBranch := r.defaultBranch(ctx, root, worktrees)
	markProtectedWorktrees(worktrees, defaultBranch)
	return worktrees, nil
}

func (r Repository) DeleteWorktree(ctx context.Context, worktree Worktree) error {
	if worktree.Protected || IsProtectedBranch(worktree.Branch) {
		return fmt.Errorf("protected branch %q cannot be deleted", worktree.Branch)
	}
	if worktree.Path == "" {
		return fmt.Errorf("delete worktree: missing path")
	}
	root, err := r.root(ctx)
	if err != nil {
		return err
	}
	if _, err := r.runner().Run(ctx, root, "git", "worktree", "remove", "--force", worktree.Path); err != nil {
		return fmt.Errorf("git worktree remove %s: %w", worktree.Path, err)
	}
	if worktree.Branch == "" || worktree.Branch == "detached" {
		return nil
	}
	if _, err := r.runner().Run(ctx, root, "git", "branch", "-D", worktree.Branch); err != nil {
		return fmt.Errorf("git branch -D %s: %w", worktree.Branch, err)
	}
	return nil
}

func (r Repository) defaultBranch(ctx context.Context, root string, worktrees []Worktree) string {
	out, err := r.runner().Run(ctx, root, "git", "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		if branch := shortRemoteBranch(trimTrailingNewline(out)); branch != "" {
			return branch
		}
	}
	out, err = r.runner().Run(ctx, root, "git", "config", "--get", "init.defaultBranch")
	if err == nil {
		if branch := trimTrailingNewline(out); branch != "" {
			return branch
		}
	}
	return inferDefaultBranch(worktrees)
}

func shortRemoteBranch(branch string) string {
	if branch == "" {
		return ""
	}
	if index := strings.IndexByte(branch, '/'); index >= 0 {
		return branch[index+1:]
	}
	return branch
}

func inferDefaultBranch(worktrees []Worktree) string {
	for _, candidate := range []string{"main", "master", "trunk"} {
		for _, worktree := range worktrees {
			if worktree.Branch == candidate {
				return candidate
			}
		}
	}
	if len(worktrees) > 0 {
		return worktrees[0].Branch
	}
	return ""
}

func markProtectedWorktrees(worktrees []Worktree, defaultBranch string) {
	for i := range worktrees {
		worktrees[i].DefaultBranch = defaultBranch != "" && worktrees[i].Branch == defaultBranch
		worktrees[i].Protected = worktrees[i].Primary || worktrees[i].Current || worktrees[i].DefaultBranch || IsProtectedBranch(worktrees[i].Branch)
	}
}

func IsProtectedBranch(branch string) bool {
	switch branch {
	case "main", "master", "develop", "dev", "production", "staging":
		return true
	}
	return strings.HasPrefix(branch, "release/") || strings.HasPrefix(branch, "hotfix/")
}

func (r Repository) runner() Runner {
	if r.Runner != nil {
		return r.Runner
	}
	return ExecRunner{}
}

func (r Repository) root(ctx context.Context) (string, error) {
	out, err := r.runner().Run(ctx, r.Dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return trimTrailingNewline(out), nil
}

func (r Repository) hasHead(ctx context.Context, dir string) bool {
	_, err := r.runner().Run(ctx, dir, "git", "rev-parse", "--verify", "HEAD")
	return err == nil
}

func trimTrailingNewline(s string) string {
	return strings.TrimRight(s, "\r\n")
}
