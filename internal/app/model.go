package app

import (
	"context"
	"fmt"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
	"github.com/overthinker1127/tui-worktree/internal/tui"
)

func LoadModel(ctx context.Context, repo Repository, themeName string) tui.Model {
	return loadModel(ctx, repo, themeName, false, 0, 0)
}

func loadModel(ctx context.Context, repo Repository, themeName string, transparent bool, width, height int) tui.Model {
	preset, err := theme.Preset(themeName)
	themeErr := err
	if err != nil {
		preset, _ = theme.Preset("tokyonight")
	}

	snapshot := loadSnapshot(ctx, repo, "", true)
	if snapshot.Error == nil {
		snapshot.Error = themeErr
	}
	return tui.NewModel(tui.Config{
		Context:          ctx,
		ThemeName:        preset.Name,
		Theme:            theme.NewStylesWithOptions(preset, theme.StyleOptions{Transparent: transparent}),
		Transparent:      transparent,
		ThemeNames:       theme.Names(),
		Width:            width,
		Height:           height,
		Worktrees:        snapshot.Worktrees,
		SelectedWorktree: snapshot.SelectedWorktree,
		Changes:          snapshot.Changes,
		Diffs:            snapshot.Diffs,
		Error:            snapshot.Error,
		LoadDiff: func(ctx context.Context, worktreePath string, change gitview.FileChange) string {
			return loadDiff(ctx, repositoryAt(repo, worktreePath), change)
		},
		Reload: func(ctx context.Context, selectedWorktreePath string) tui.Snapshot {
			return loadSnapshot(ctx, repo, selectedWorktreePath, false)
		},
		DeleteWorktree: func(ctx context.Context, worktree gitview.Worktree) error {
			if deleteRepo, ok := repo.(DeleteWorktreeRepository); ok {
				return deleteRepo.DeleteWorktree(ctx, worktree)
			}
			return fmt.Errorf("delete worktree is not supported")
		},
		SaveTheme: func(name string) error {
			cfg, _ := LoadConfig()
			cfg.Theme = name
			return SaveConfig(cfg)
		},
		SaveTransparent: func(transparent bool) error {
			cfg, _ := LoadConfig()
			cfg.Transparent = transparent
			return SaveConfig(cfg)
		},
	})
}

func loadSnapshot(ctx context.Context, repo Repository, selectedWorktreePath string, includeDiff bool) tui.Snapshot {
	worktrees, err := loadWorktrees(ctx, repo)
	if err != nil {
		return tui.Snapshot{Error: err}
	}

	states := make([]tui.WorktreeState, 0, len(worktrees))
	selected := 0
	for i, worktree := range worktrees {
		if worktree.Path == selectedWorktreePath || selectedWorktreePath == "" && worktree.Current {
			selected = i
		}
		changes, err := repositoryAt(repo, worktree.Path).Changes(ctx)
		states = append(states, tui.WorktreeState{
			Worktree: worktree,
			Changes:  changes,
			Error:    err,
		})
	}
	if len(states) == 0 {
		return tui.Snapshot{}
	}

	changes := states[selected].Changes
	var diffs map[string]string
	if includeDiff && len(changes) > 0 {
		diffs = make(map[string]string, 1)
		key := states[selected].Worktree.Path + "\x00" + changes[0].Path
		diffs[key] = loadDiff(ctx, repositoryAt(repo, states[selected].Worktree.Path), changes[0])
	}
	return tui.Snapshot{
		Worktrees:        states,
		SelectedWorktree: selected,
		Changes:          changes,
		Diffs:            diffs,
		Error:            states[selected].Error,
	}
}
