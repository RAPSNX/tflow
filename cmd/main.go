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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "menu":
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
	return ui.Start(ctx)
}
