package main

import (
	"fmt"
	"log"
	"os"

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
			return ui.OpenMenu()
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
