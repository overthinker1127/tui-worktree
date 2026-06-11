package app

import (
	"context"
	"flag"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"

	gitview "github.com/overthinker1127/tui-worktree/internal/git"
	"github.com/overthinker1127/tui-worktree/internal/theme"
	"github.com/overthinker1127/tui-worktree/internal/tui"
)

type Options struct {
	Dir   string
	Theme string
}

const usage = `Usage:
  tui-worktree [--repo PATH] [--theme NAME]

Themes:
  dark, light, tokyonight, tokyonight-night, tokyonight-storm, kanagawa, kanagawa-wave, kanagawa-dragon
`

type Repository interface {
	Changes(context.Context) ([]gitview.FileChange, error)
	Diff(context.Context, gitview.FileChange) (string, error)
}

func ParseArgs(args []string) (Options, error) {
	fs := flag.NewFlagSet("tui-worktree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := Options{Dir: ".", Theme: "tokyonight"}
	fs.StringVar(&opts.Dir, "repo", opts.Dir, "repository path")
	fs.StringVar(&opts.Theme, "theme", opts.Theme, "theme preset: dark, light, tokyonight, tokyonight-storm, kanagawa, kanagawa-dragon")
	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	return opts, nil
}

func Usage() string {
	return usage
}

func LoadModel(ctx context.Context, repo Repository, themeName string) tui.Model {
	preset, err := theme.Preset(themeName)
	if err != nil {
		preset, _ = theme.Preset("tokyonight")
		return tui.NewModel(tui.Config{
			Theme: theme.NewStyles(preset),
			Error: err,
		})
	}

	changes, err := repo.Changes(ctx)
	if err != nil {
		return tui.NewModel(tui.Config{
			Theme: theme.NewStyles(preset),
			Error: err,
		})
	}

	diffs := make(map[string]string, len(changes))
	for _, change := range changes {
		diff, err := repo.Diff(ctx, change)
		if err != nil {
			diff = fmt.Sprintf("Could not load diff for %s:\n%s", change.Path, err)
		}
		diffs[change.Path] = diff
	}

	return tui.NewModel(tui.Config{
		Theme:   theme.NewStyles(preset),
		Changes: changes,
		Diffs:   diffs,
	})
}

func Run(ctx context.Context, opts Options) error {
	repo := gitview.Repository{Dir: opts.Dir}
	model := LoadModel(ctx, repo, opts.Theme)
	_, err := tea.NewProgram(model).Run()
	return err
}
