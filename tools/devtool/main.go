package main

import (
	"context"
	"os"
	"os/signal"

	"discrescue/tools/devtool/internal/devtool"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(devtool.Main(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
