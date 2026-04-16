package components

import (
	"strings"

	bubblesKeys "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func returnPackageKeyMap() []Bind {
	return []Bind{
		Bind{
			Binding: bubblesKeys.NewBinding(
				bubblesKeys.WithKeys("q", "ctrl+c"),
				bubblesKeys.WithKeys("q or ctrl+c to escape"),
			),
			Handler: func() {
				tea.Quit()
			},
		},
	}
}

func RenderPackagePanel(w, h int, pkgs []string) (string, []Bind) {
	packagesStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).Width(w - 4).Height(h - 3)

	var lines []string
	for _, p := range pkgs {
		lines = append(lines, "  "+p)
	}
	packagesList := packagesStyle.Render(strings.Join(lines, "\n"))

	// Return rendering and an optional key binding -> handler map for future interactivity.
	// For now, we don't bind any handlers, so return nil.
	return packagesList, returnPackageKeyMap()
}
