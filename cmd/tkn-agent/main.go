package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift-pipelines/tkn-agent/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], cli.IO{Out: os.Stdout, Err: os.Stderr})
	stop()
	os.Exit(code)
}
