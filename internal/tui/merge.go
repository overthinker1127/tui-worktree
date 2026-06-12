package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func defaultMergeBranch(ctx context.Context, req MergeRequest) error {
	if req.Source.Branch == "" || req.Source.Branch == "detached" {
		return fmt.Errorf("selected worktree has no branch")
	}
	if req.Target.Path == "" {
		return fmt.Errorf("merge target has no worktree path")
	}
	cmd := exec.CommandContext(ctx, "git", "merge", "--no-edit", req.Source.Branch)
	cmd.Dir = req.Target.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("git merge %s: %w: %s", req.Source.Branch, err, detail)
		}
		return fmt.Errorf("git merge %s: %w", req.Source.Branch, err)
	}
	return nil
}
