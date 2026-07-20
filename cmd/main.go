package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	runtmux "tflow/internal/tmux"
	"tflow/internal/ui"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "menu":
			ctx, stop := signalContext()
			defer stop()
			return ui.OpenMenu(ctx)
		case "open-quit":
			return ui.OpenQuit()
		case "toggle-menu":
			return ui.ToggleMenu()
		case "create-worker":
			if len(args) != 2 {
				return fmt.Errorf("create-worker requires one payload")
			}
			return ui.RunCreateWorker(args[1])
		case "remember-client":
			return runtmux.RememberCurrentClient()
		case "cleanup-client":
			return runtmux.CleanupDetachedClient()
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
	}
	ctx, stop := signalContext()
	defer stop()
	return ui.Start(ctx)
}

// signalContext is installed only for the long-running runtime boundaries
// (the attached tmux client and the Bubble Tea popup program) that observe
// cancellation. Other subcommands are short-lived, non-cancellation-aware
// child invocations; installing a signal handler for them would intercept
// SIGHUP/SIGINT/SIGTERM without anything reacting to it, leaving them
// unkillable by signal until they happen to finish on their own.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
}
