package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"

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

func ParseArgs(args []string) (Options, error) {
	fs := flag.NewFlagSet("tui-worktree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := Options{Dir: ".", Theme: "tokyonight"}
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
	preset, err := theme.Preset(themeName)
	themeErr := err
	if err != nil {
		preset, _ = theme.Preset("tokyonight")
	}

	snapshot := loadSnapshot(ctx, repo)
	if snapshot.Error == nil {
		snapshot.Error = themeErr
	}
	return tui.NewModel(tui.Config{
		Context:    ctx,
		ThemeName:  preset.Name,
		Theme:      theme.NewStyles(preset),
		ThemeNames: theme.Names(),
		Changes:    snapshot.Changes,
		Diffs:      snapshot.Diffs,
		Error:      snapshot.Error,
		LoadDiff: func(ctx context.Context, change gitview.FileChange) string {
			return loadDiff(ctx, repo, change)
		},
		Reload: func(ctx context.Context) tui.Snapshot {
			return loadSnapshot(ctx, repo)
		},
	})
}

func loadSnapshot(ctx context.Context, repo Repository) tui.Snapshot {
	changes, err := repo.Changes(ctx)
	if err != nil {
		return tui.Snapshot{Error: err}
	}

	diffs := make(map[string]string, min(1, len(changes)))
	if len(changes) > 0 {
		diffs[changes[0].Path] = loadDiff(ctx, repo, changes[0])
	}
	return tui.Snapshot{Changes: changes, Diffs: diffs}
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
	model := LoadModel(ctx, repo, opts.Theme)
	_, err := tea.NewProgram(model).Run()
	return err
}
