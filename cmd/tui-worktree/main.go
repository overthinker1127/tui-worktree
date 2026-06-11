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
			fmt.Print(app.Usage("tui-worktree"))
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := app.Run(context.Background(), opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
