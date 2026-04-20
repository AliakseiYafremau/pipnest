package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Rotlerxd/pipnest/internal/backends"
	"github.com/Rotlerxd/pipnest/internal/venv"
)

func RenderSearchBox(query string, width int) string {
	if width < 8 {
		width = 8
	}
	label := "Search"
	value := query
	if strings.TrimSpace(value) == "" {
		value = lipgloss.NewStyle().Foreground(UI.MutedTextColor).Render("type to filter...")
	}
	line := strings.Repeat("-", width)
	return label + "\n" + value + "\n" + line
}

func RenderPackagesList(packages []backends.Package, selected, maxLines int) string {
	if len(packages) == 0 {
		return lipgloss.NewStyle().Foreground(UI.MutedTextColor).Render("No installed packages")
	}
	if maxLines <= 0 {
		maxLines = len(packages)
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= len(packages) {
		selected = len(packages) - 1
	}

	start := 0
	if selected >= maxLines {
		start = selected - maxLines + 1
	}
	end := start + maxLines
	if end > len(packages) {
		end = len(packages)
	}

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		line := fmt.Sprintf("%s %s", packages[i].Name, packages[i].Version)
		if strings.TrimSpace(packages[i].Version) == "" {
			line = packages[i].Name
		}

		if i == selected {
			line = lipgloss.NewStyle().Foreground(UI.SelectedColor).Render("> " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func RenderVenvList(venvs []venv.Venv, maxLines int) string {
	if len(venvs) == 0 {
		return lipgloss.NewStyle().Foreground(UI.MutedTextColor).Render("No .venv found")
	}
	if maxLines <= 0 || maxLines > len(venvs) {
		maxLines = len(venvs)
	}

	lines := make([]string, 0, maxLines)
	for _, v := range venvs[:maxLines] {
		if strings.TrimSpace(v.Path) == "" {
			lines = append(lines, "- "+v.Name)
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s (%s)", v.Name, v.Path))
	}

	return strings.Join(lines, "\n")
}
