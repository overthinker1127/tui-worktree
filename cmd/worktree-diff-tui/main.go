package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/overthinker1127/tui-worktree/internal/app"
)

func main() {
	opts, err := app.ParseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(app.Usage("worktree-diff-tui"))
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if opts.Version {
		fmt.Print(app.Version("worktree-diff-tui"))
		return
	}
	if err := app.Run(context.Background(), opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
