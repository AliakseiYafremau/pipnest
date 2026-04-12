//go:build linux || darwin
// +build linux darwin

package packages

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderResults renders the package search result list pane.
func RenderResults(results []Result, width int, selectedIndex int) string {
	if len(results) == 0 {
		return ""
	}

	if width < 20 {
		width = 20
	}

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("57"))

	headerStyle := lipgloss.NewStyle().Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var lines []string
	lines = append(lines, headerStyle.Render("Packages"))
	lines = append(lines, subtitleStyle.Render("Arrows or click to highlight"))
	lines = append(lines, "")

	for i, result := range results {
		line := formatResultLine(result, width)
		if i == selectedIndex {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func formatResultLine(result Result, width int) string {
	nameStyle := lipgloss.NewStyle().Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	line := nameStyle.Render(result.Name)
	if result.Version != "" {
		line += " " + metaStyle.Render(result.Version)
	}

	if result.Description != "" {
		summaryWidth := width - lipgloss.Width(result.Name) - 3
		if summaryWidth < 18 {
			summaryWidth = 18
		}
		line += metaStyle.Render(" - " + TruncateText(strings.TrimSpace(result.Description), summaryWidth))
	}

	if lipgloss.Width(line) > width {
		line = TruncateText(line, width)
	}
	return line
}

// SelectedSearchResult returns the currently selected result or nil.
func SelectedSearchResult(results []Result, index int) *Result {
	if index < 0 || index >= len(results) {
		return nil
	}
	return &results[index]
}

// RenderPackageDetails renders the package details pane content.
func RenderPackageDetails(result *Result, width int, loading bool, query string, err error) string {
	if width < 24 {
		width = 24
	}

	titleStyle := lipgloss.NewStyle().Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("230"))

	var lines []string
	lines = append(lines, titleStyle.Render("Package Info"))
	lines = append(lines, metaStyle.Render(strings.TrimSpace(query)))
	lines = append(lines, "")

	if err != nil {
		lines = append(lines, metaStyle.Render("Search error:"))
		lines = append(lines, WrapText(err.Error(), width))
		return strings.Join(lines, "\n")
	}

	if loading && result == nil {
		lines = append(lines, metaStyle.Render("⠋ Loading results..."))
		return strings.Join(lines, "\n")
	}

	if result == nil {
		lines = append(lines, metaStyle.Render("Select a package on the left."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, valueStyle.Render(result.Name))
	if result.Version != "" {
		lines = append(lines, metaStyle.Render("Version"))
		lines = append(lines, WrapText(result.Version, width))
	}
	if result.Description != "" {
		lines = append(lines, metaStyle.Render("Summary"))
		lines = append(lines, WrapText(result.Description, width))
	}
	if result.URL != "" {
		lines = append(lines, metaStyle.Render("Project URL"))
		lines = append(lines, WrapText(result.URL, width))
	}

	return strings.Join(lines, "\n")
}

// TruncateText truncates text to max runes and appends an ellipsis when needed.
func TruncateText(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

// WrapText wraps plain text at the given width.
func WrapText(text string, width int) string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	var line strings.Builder
	for _, word := range words {
		if line.Len() == 0 {
			line.WriteString(word)
			continue
		}
		if line.Len()+1+len(word) > width {
			lines = append(lines, line.String())
			line.Reset()
			line.WriteString(word)
			continue
		}
		line.WriteByte(' ')
		line.WriteString(word)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}
