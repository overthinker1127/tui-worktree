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
)

type Options struct {
	Dir         string
	Theme       string
	Transparent bool
	Version     bool
}

var BuildVersion = "dev"

func ParseArgs(args []string) (Options, error) {
	fs := flag.NewFlagSet("tui-worktree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := Options{Dir: "."}
	fs.StringVar(&opts.Dir, "repo", opts.Dir, "repository path")
	fs.StringVar(&opts.Theme, "theme", opts.Theme, "theme preset: "+strings.Join(theme.Names(), ", "))
	fs.BoolVar(&opts.Transparent, "transparent", opts.Transparent, "do not paint theme background colors")
	fs.BoolVar(&opts.Version, "version", opts.Version, "print version and exit")
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
  %s [--repo PATH] [--theme NAME] [--transparent] [--version]

Themes:
  %s
`, command, strings.Join(theme.Names(), ", "))
}

func Version(command string) string {
	if command == "" {
		command = "tui-worktree"
	}
	return fmt.Sprintf("%s %s\n", command, BuildVersion)
}

func Run(ctx context.Context, opts Options) error {
	repo := gitview.Repository{Dir: opts.Dir}
	if err := ensureRepository(ctx, repo, opts.Dir); err != nil {
		return err
	}
	width, height := terminalSize()
	model := loadModel(ctx, repo, ResolveTheme(opts), ResolveTransparent(opts), width, height)
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
