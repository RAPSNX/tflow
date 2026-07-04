package main

import (
	"fmt"
	"log"
	"os"

	"tflow/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
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
			return runProgram(ui.NewMenu())
		case "toggle-menu":
			return ui.ToggleMenu()
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
	}
	return ui.Start()
}

func runProgram(m tea.Model) error {
	program := tea.NewProgram(m, tea.WithAltScreen())
	_, err := program.Run()
	return err
}
