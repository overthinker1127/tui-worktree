package app

import (
	"context"
	"fmt"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
)

type Repository interface {
	Changes(context.Context) ([]gitview.FileChange, error)
	Diff(context.Context, gitview.FileChange) (string, error)
}

type WorktreeRepository interface {
	Worktrees(context.Context) ([]gitview.Worktree, error)
}

type RootRepository interface {
	Root(context.Context) (string, error)
}

type DeleteWorktreeRepository interface {
	DeleteWorktree(context.Context, gitview.Worktree) error
}

func loadWorktrees(ctx context.Context, repo Repository) ([]gitview.Worktree, error) {
	if worktreeRepo, ok := repo.(WorktreeRepository); ok {
		return worktreeRepo.Worktrees(ctx)
	}
	return []gitview.Worktree{{Path: ".", Branch: "current", Current: true}}, nil
}

func repositoryAt(repo Repository, worktreePath string) Repository {
	if worktreePath == "" {
		return repo
	}
	switch typed := repo.(type) {
	case gitview.Repository:
		typed.Dir = worktreePath
		return typed
	case *gitview.Repository:
		next := *typed
		next.Dir = worktreePath
		return next
	default:
		return repo
	}
}

func loadDiff(ctx context.Context, repo Repository, change gitview.FileChange) string {
	diff, err := repo.Diff(ctx, change)
	if err != nil {
		return fmt.Sprintf("Could not load diff for %s:\n%s", change.Path, err)
	}
	return diff
}

func ensureRepository(ctx context.Context, repo RootRepository, dir string) error {
	if _, err := repo.Root(ctx); err != nil {
		if dir == "" {
			dir = "."
		}
		return fmt.Errorf("not a git repository: %s", dir)
	}
	return nil
}
