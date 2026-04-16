package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Rotlerxd/pipnest/internal/service"
	"github.com/Rotlerxd/pipnest/internal/ui"
)

func main() {
	svc, err := service.NewService("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to initialize service:", err)
		os.Exit(1)
	}

	m := ui.NewAppModel(svc)

	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to start TUI:", err)
		os.Exit(2)
	}
}
