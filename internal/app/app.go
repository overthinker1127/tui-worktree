package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
	"github.com/overthinker1127/tui-worktree/internal/tui"
)

type Options struct {
	Dir   string
	Theme string
}

type Repository interface {
	Changes(context.Context) ([]gitview.FileChange, error)
	Diff(context.Context, gitview.FileChange) (string, error)
}

type WorktreeRepository interface {
	Worktrees(context.Context) ([]gitview.Worktree, error)
}

type DeleteWorktreeRepository interface {
	DeleteWorktree(context.Context, gitview.Worktree) error
}

func ParseArgs(args []string) (Options, error) {
	fs := flag.NewFlagSet("tui-worktree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := Options{Dir: "."}
	fs.StringVar(&opts.Dir, "repo", opts.Dir, "repository path")
	fs.StringVar(&opts.Theme, "theme", opts.Theme, "theme preset: "+strings.Join(theme.Names(), ", "))
	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	return opts, nil
}

func Usage(command string) string {
	if command == "" {
		command = "tui-worktree"
	}
	return fmt.Sprintf(`Usage:
  %s [--repo PATH] [--theme NAME]

Themes:
  %s
`, command, strings.Join(theme.Names(), ", "))
}

func LoadModel(ctx context.Context, repo Repository, themeName string) tui.Model {
	return loadModel(ctx, repo, themeName, 0, 0)
}

func loadModel(ctx context.Context, repo Repository, themeName string, width, height int) tui.Model {
	preset, err := theme.Preset(themeName)
	themeErr := err
	if err != nil {
		preset, _ = theme.Preset("tokyonight")
	}

	snapshot := loadSnapshot(ctx, repo, "")
	if snapshot.Error == nil {
		snapshot.Error = themeErr
	}
	return tui.NewModel(tui.Config{
		Context:          ctx,
		ThemeName:        preset.Name,
		Theme:            theme.NewStyles(preset),
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
			return loadSnapshot(ctx, repo, selectedWorktreePath)
		},
		DeleteWorktree: func(ctx context.Context, worktree gitview.Worktree) error {
			if deleteRepo, ok := repo.(DeleteWorktreeRepository); ok {
				return deleteRepo.DeleteWorktree(ctx, worktree)
			}
			return fmt.Errorf("delete worktree is not supported")
		},
		SaveTheme: func(name string) error {
			return SaveConfig(UserConfig{Theme: name})
		},
	})
}

func loadSnapshot(ctx context.Context, repo Repository, selectedWorktreePath string) tui.Snapshot {
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
	diffs := make(map[string]string, min(1, len(changes)))
	if len(changes) > 0 {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Run(ctx context.Context, opts Options) error {
	repo := gitview.Repository{Dir: opts.Dir}
	width, height := terminalSize()
	model := loadModel(ctx, repo, ResolveTheme(opts), width, height)
	options := []tea.ProgramOption{}
	if width > 0 && height > 0 {
		options = append(options, tea.WithWindowSize(width, height))
	}
	_, err := tea.NewProgram(model, options...).Run()
	return err
}

func terminalSize() (int, int) {
	width, height, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		return 0, 0
	}
	return width, height
}
