package main

import (
	"flag"
	"fmt"
	"os"

	"pipnest/internal/venvs"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	screen := flag.String("screen", "search", "screen to open: search or venvs")
	flag.Parse()

	var program *tea.Program
	switch *screen {
	case "venvs":
		m := venvs.NewModel()
		program = tea.NewProgram(&m, tea.WithAltScreen())
	default:
		program = tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	}

	finalModel, err := program.Run()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if activatable, ok := finalModel.(interface{ ActivationCommand() string }); ok {
		if command := activatable.ActivationCommand(); command != "" {
			if err := clipboard.WriteAll(command); err != nil {
				fmt.Println("Could not copy activation command to clipboard. Run this manually:")
				fmt.Println(command)
			} else {
				if withMessage, ok := finalModel.(interface{ ActivationMessage() string }); ok {
					message := withMessage.ActivationMessage()
					if message != "" {
						fmt.Println(message)
					} else {
						fmt.Println("Activation command copied to clipboard. Paste and run it in your shell.")
					}
				} else {
					fmt.Println("Activation command copied to clipboard. Paste and run it in your shell.")
				}
			}
		}
	}
}
