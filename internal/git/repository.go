package git

import (
	"context"
	"fmt"
	"os/exec"
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
	statusOut, err := runner.Run(ctx, r.Dir, "git", "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	changes, err := ParsePorcelainStatus(statusOut)
	if err != nil {
		return nil, err
	}

	numstatOut, err := runner.Run(ctx, r.Dir, "git", "diff", "--numstat", "-z", "HEAD", "--")
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
	args := []string{"diff", "--no-ext-diff", "--find-renames", "--color=always", "HEAD", "--"}
	if change.Status == Renamed && change.OldPath != "" {
		args = append(args, change.OldPath, change.Path)
	} else {
		args = append(args, change.Path)
	}
	out, err := r.runner().Run(ctx, r.Dir, "git", args...)
	if err != nil {
		return "", fmt.Errorf("git diff %s: %w", change.Path, err)
	}
	if out == "" {
		return fmt.Sprintf("No diff for %s", change.Path), nil
	}
	return out, nil
}

func (r Repository) Root(ctx context.Context) (string, error) {
	out, err := r.runner().Run(ctx, r.Dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return trimTrailingNewline(out), nil
}

func (r Repository) runner() Runner {
	if r.Runner != nil {
		return r.Runner
	}
	return ExecRunner{}
}

func trimTrailingNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
