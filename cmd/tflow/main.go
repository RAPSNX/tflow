package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rapsnx/tflow/internal/buildinfo"
	runtmux "github.com/rapsnx/tflow/internal/tmux"
	"github.com/rapsnx/tflow/internal/ui"
)

const helpText = `tflow is a focused tmux-backed terminal session manager.

Usage:
  tflow [command]

Without a command, tflow starts a new tflow instance.

Commands:
  version, --version  Print version information
  help, -h, --help    Show this help text
`

func main() {
	if err := run(os.Args[1:], os.Stdout, ui.Start); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, output io.Writer, start func() error) error {
	if len(args) == 0 {
		return start()
	}

	switch args[0] {
	case "help", "-h", "--help":
		_, err := fmt.Fprint(output, helpText)
		return err
	case "version", "--version":
		_, err := fmt.Fprintln(output, "tflow", buildinfo.Version())
		return err
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
		return fmt.Errorf("unknown command %q; run %q for usage", args[0], "tflow --help")
	}
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
