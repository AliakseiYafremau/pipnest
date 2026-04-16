package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Rotlerxd/pipnest/internal/service"
	"github.com/Rotlerxd/pipnest/internal/ui"
)

func main() {
	pythonPath := flag.String("python", "python", "path to python executable (used to detect backends)")
	flag.Parse()

	svc, err := service.NewService(*pythonPath)
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
