package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	devtool "discrescue/tools/devtool/internal/devtool"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := devtool.New(os.Stdout, os.Stderr)
	if err := app.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
