package components

import (
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss"
)

type PackagesKeyMap struct{}

// RenderPackagePanel returns a simple package panel rendering. It is a
// small, self-contained helper so other UI files can use it without
// introducing compile-time dependencies on runtime layout variables.
func RenderPackagePanel(w, h int, pkgs []string) string {
	packagesStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).Width(w - 4).Height(h - 3)

	var lines []string
	for _, p := range pkgs {
		lines = append(lines, "  "+p)
	}
	packagesList := packagesStyle.Render(strings.Join(lines, "\n"))

	return packagesList
}
