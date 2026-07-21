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
		case "cleanup-client":
			return runtmux.CleanupDetachedClient()
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
	}
	return ui.Start()
}

// signalContext is installed only for the long-running Bubble Tea popup
// program, which observes cancellation directly and does no blocking,
// non-context-aware setup beforehand. The default (attach) path builds its
// own signal-aware context internally, at the attach boundary, rather than
// here: prepareStartup runs several synchronous, non-context-aware tmux
// setup calls first, and installing a signal handler before those run would
// disable the Go runtime's default terminate-on-signal behavior for the
// whole process while nothing is watching that context yet. Other
// subcommands are short-lived, non-cancellation-aware child invocations;
// installing a signal handler for them would intercept
// SIGHUP/SIGINT/SIGTERM without anything reacting to it, leaving them
// unkillable by signal until they happen to finish on their own.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
}
