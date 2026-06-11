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

func Usage() string {
	return fmt.Sprintf(`Usage:
  tui-worktree [--repo PATH] [--theme NAME]

Themes:
  %s
`, strings.Join(theme.Names(), ", "))
}

func LoadModel(ctx context.Context, repo Repository, themeName string) tui.Model {
	preset, err := theme.Preset(themeName)
	if err != nil {
		preset, _ = theme.Preset("tokyonight")
		return tui.NewModel(tui.Config{
			ThemeName:  "tokyonight",
			Theme:      theme.NewStyles(preset),
			ThemeNames: theme.Names(),
			Error:      err,
			Reload: func(ctx context.Context) tui.Snapshot {
				return loadSnapshot(ctx, repo)
			},
		})
	}

	snapshot := loadSnapshot(ctx, repo)
	return tui.NewModel(tui.Config{
		ThemeName:  preset.Name,
		Theme:      theme.NewStyles(preset),
		ThemeNames: theme.Names(),
		Changes:    snapshot.Changes,
		Diffs:      snapshot.Diffs,
		Error:      snapshot.Error,
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

	diffs := make(map[string]string, len(changes))
	for _, change := range changes {
		diff, err := repo.Diff(ctx, change)
		if err != nil {
			diff = fmt.Sprintf("Could not load diff for %s:\n%s", change.Path, err)
		}
		diffs[change.Path] = diff
	}
	return tui.Snapshot{Changes: changes, Diffs: diffs}
}

func Run(ctx context.Context, opts Options) error {
	repo := gitview.Repository{Dir: opts.Dir}
	model := LoadModel(ctx, repo, opts.Theme)
	_, err := tea.NewProgram(model).Run()
	return err
}
