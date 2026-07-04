package main

import (
	"fmt"
	"log"
	"os"

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
			_, err := ui.RunMenu("")
			return err
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
	}
	return ui.Start()
}
