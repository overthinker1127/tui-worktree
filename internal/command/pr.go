package command

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func FindForgeCLI() (string, bool) {
	for _, name := range []string{"gh", "glab"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, true
		}
	}
	return "", false
}

func CreatePullRequest(ctx context.Context, req PullRequestRequest) error {
	args, err := PullRequestCreateArgs(req)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, req.CLI, args...)
	if req.WorktreeDir != "" {
		cmd.Dir = req.WorktreeDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("%s %v: %w: %s", req.CLI, args, err, detail)
		}
		return fmt.Errorf("%s %v: %w", req.CLI, args, err)
	}
	return nil
}

func PullRequestCreateArgs(req PullRequestRequest) ([]string, error) {
	switch req.CLI {
	case "gh":
		return []string{"pr", "create", "--title", req.Title, "--body", req.Body, "--head", req.Branch}, nil
	case "glab":
		return []string{"mr", "create", "--title", req.Title, "--description", req.Body, "--source-branch", req.Branch, "--yes"}, nil
	default:
		return nil, fmt.Errorf("unsupported Forge CLI %q", req.CLI)
	}
}
