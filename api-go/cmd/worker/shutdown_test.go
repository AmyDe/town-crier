package main

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

// Not parallel: it signals the test process itself.
func TestShutdownContext_SIGTERMCancelsTheRunContext(t *testing.T) {
	ctx, stop := shutdownContext(context.Background())
	defer stop()

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("SIGTERM did not cancel the run context")
	}
}
