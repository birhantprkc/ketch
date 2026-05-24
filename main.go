package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/1broseidon/ketch/cmd"
)

func main() {
	// Cancel the root context on SIGINT/SIGTERM so foreground commands
	// (notably `ketch crawl`) can shut down gracefully: workers exit,
	// in-flight HTTP requests abort, and the process returns instead of
	// being hard-killed by the default signal handler.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err := cmd.ExecuteContext(ctx)
	if err == nil {
		return
	}

	// Translate classified errors to documented exit codes (see cmd/exit.go).
	// Cancellation is checked first so a SIGINT that propagated through any
	// error path still exits 6 regardless of how it was wrapped.
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		os.Exit(cmd.ExitCancelled)
	}
	var exitErr *cmd.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}
	os.Exit(1)
}
