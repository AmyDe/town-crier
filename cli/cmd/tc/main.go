// Command tc is the Town Crier admin CLI: generate offer codes, grant
// subscriptions, and list users against the admin API.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/AmyDe/town-crier/cli/internal/tc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	env := tc.Env{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
	os.Exit(tc.Run(ctx, env, os.Args[1:]))
}
