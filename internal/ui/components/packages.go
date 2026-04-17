package components

import (
	"strings"

	bubbles "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

func returnPackageKeyMap() []Bind {
	return []Bind{
		{
			Binding: bubbles.NewBinding(
				bubbles.WithKeys("q", "ctrl+c"),
				bubbles.WithHelp("q/ctrl+c", "quit"),
			),
			Handler: func() {
				// no-op for now; Update() will handle quitting via exitKeyMap.
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

	return packagesList, returnPackageKeyMap()
}
