package venvs

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.view.Width <= 0 || m.view.Height <= 0 {
		return ""
	}
	if m.view.Width < minWidth || m.view.Height < minHeight {
		return m.renderInsufficientSpace()
	}

	bodyHeight := m.view.Height - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	panelHeight := bodyHeight - 1
	if panelHeight < 1 {
		panelHeight = 1
	}
	contentWidth := m.view.Width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	leftWidth, rightWidth := splitTwoWidths(contentWidth)
	leftPanel := m.renderLeftPanel(leftWidth, panelHeight)
	rightPanel := m.renderDetailsAndPackagesPanel(rightWidth, panelHeight)
	legend := m.renderLegend()

	row := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, lipgloss.NewStyle().Width(3).Render(""), rightPanel)
	ui := lipgloss.Place(m.view.Width, bodyHeight, lipgloss.Center, lipgloss.Top, row)

	if m.replModalOpen {
		ui = m.renderREPLModal()
	}
	return ui + "\n" + legend
}

func (m Model) renderInsufficientSpace() string {
	message := strings.Join([]string{
		"Not enough terminal space",
		fmt.Sprintf("Current: %dx%d", m.view.Width, m.view.Height),
		fmt.Sprintf("Minimum: %dx%d", minWidth, minHeight),
		"Resize the terminal to continue.",
	}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#999999")).
		Padding(1, 2).
		Render(message)

	return lipgloss.Place(m.view.Width, m.view.Height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderLeftPanel(width, height int) string {
	focused := !m.focusPackages
	innerHeight := max(1, height-4)
	maxW := max(1, width-4)

	currentLabel := "Current environment"
	currentValue := "No active interpreter"
	if m.view.Interpreter != "" {
		currentValue = filepath.Base(m.view.Interpreter)
	}

	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	lines := []string{
		muted.Render(truncateLine(currentLabel, maxW)),
		StyleForInterpreter(m.view.InterpreterKind).Render(truncateLine(currentValue, maxW)),
		"",
		lipgloss.NewStyle().Bold(true).Render(truncateLine("Interpreter dropdown", maxW)),
	}

	if m.dropdownOpen {
		maxWidth := max(1, width-8)
		availableRows := innerHeight - len(lines)
		if availableRows < 0 {
			availableRows = 0
		}
		start := 0
		end := len(m.interpreters)
		if availableRows < len(m.interpreters) {
			start = clamp(m.selected-(availableRows/2), 0, max(0, len(m.interpreters)-availableRows))
			end = start + availableRows
		}
		for i := start; i < end; i++ {
			option := m.interpreters[i]
			label := option.Label
			if option.Path != "" {
				label = fmt.Sprintf("%s - %s", option.Label, option.Path)
			}
			label = truncateLine(label, maxWidth)
			var lineStyle lipgloss.Style
			if i == m.selected && focused {
				lineStyle = lipgloss.NewStyle().Background(lipgloss.Color(focusHighlightColor)).Foreground(lipgloss.Color("230"))
				label = "> " + label
			} else {
				lineStyle = lipgloss.NewStyle()
				label = "  " + label
			}
			lines = append(lines, lineStyle.Render(label))
		}
	} else {
		lines = append(lines, muted.Render(truncateLine("Press Enter to open", maxW)))
	}

	lines = fillToHeight(lines, innerHeight)
	return panelStyle(focused).
		Padding(1, 1).
		Width(width).
		Height(innerHeight).
		Render(strings.Join(lines, "\n"))
}

func (m *Model) renderDetailsAndPackagesPanel(width, height int) string {
	details := m.highlighted
	innerHeight := max(1, height-4)
	maxW := max(1, width-4)

	kind := details.Kind
	if kind == "" {
		kind = InterpreterGlobal
	}
	path := details.Path
	if path == "" {
		path = "No interpreter selected"
	}

	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	lines := []string{
		muted.Render(truncateLine("Highlighted interpreter", maxW)),
		StyleForInterpreter(kind).Render(truncateLine(path, maxW)),
		muted.Render(truncateLine("Version: "+valueOrUnknown(details.Version), maxW)),
		muted.Render(truncateLine("Size: "+valueOrUnknown(details.SizeLabel), maxW)),
		muted.Render(truncateLine("Created: "+valueOrUnknown(details.CreatedAtLabel), maxW)),
		muted.Render(truncateLine("Updated: "+valueOrUnknown(details.UpdatedAtLabel), maxW)),
		muted.Render(truncateLine(fmt.Sprintf("Packages: %d", details.PackageCount), maxW)),
		muted.Render(truncateLine("Cmd: "+valueOrUnknown(details.ActivationCommand), maxW)),
		"",
		muted.Render(truncateLine("Installed packages", maxW)),
	}

	if len(details.Packages) == 0 {
		if len(lines) < innerHeight {
			lines = append(lines, muted.Render("No packages found"))
		}
	} else {
		availableRows := packageVisibleLines(innerHeight, len(lines))
		displayRows := availableRows
		if len(details.Packages) > availableRows && availableRows > 1 {
			displayRows = availableRows - 1
		}
		if displayRows < 0 {
			displayRows = 0
		}
		if m.packageSelected >= len(details.Packages) {
			m.packageSelected = len(details.Packages) - 1
		}
		if m.packageSelected < 0 {
			m.packageSelected = 0
		}
		if displayRows > 0 {
			if m.packageScroll > m.packageSelected {
				m.packageScroll = m.packageSelected
			}
			if m.packageSelected >= m.packageScroll+displayRows {
				m.packageScroll = m.packageSelected - displayRows + 1
			}
			if m.packageSelected < m.packageScroll {
				m.packageScroll = m.packageSelected
			}
		}
		end := m.packageScroll + displayRows
		if end > len(details.Packages) {
			end = len(details.Packages)
		}
		pkgMaxW := max(1, maxW-2)
		for i := m.packageScroll; i < end && len(lines) < innerHeight; i++ {
			item := details.Packages[i]
			label := truncateLine(fmt.Sprintf("%s %s", item.Name, item.Version), pkgMaxW)
			selected := i == m.packageSelected && m.focusPackages
			if selected {
				label = "> " + label
			} else {
				label = "  " + label
			}
			var lineStyle lipgloss.Style
			if selected {
				lineStyle = lipgloss.NewStyle().Background(lipgloss.Color(focusHighlightColor)).Foreground(lipgloss.Color("230"))
			} else {
				lineStyle = lipgloss.NewStyle()
			}
			lines = append(lines, lineStyle.Render(label))
		}
		if remaining := len(details.Packages) - end; remaining > 0 && len(lines) < innerHeight {
			lines = append(lines, muted.Render(fmt.Sprintf("+%d more", remaining)))
		}
	}

	lines = fillToHeight(lines, innerHeight)
	return panelStyle(m.focusPackages).
		Padding(1, 1).
		Width(width).
		Height(innerHeight).
		Render(strings.Join(lines, "\n"))
}

func (m Model) renderLegend() string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f2f2f2"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))

	leftLegend := lipgloss.JoinHorizontal(lipgloss.Top,
		keyStyle.Render("Enter"), sepStyle.Render(": select"),
		sepStyle.Render("  |  "),
		keyStyle.Render("←/→"), sepStyle.Render(": focus"),
		sepStyle.Render("  |  "),
		keyStyle.Render("j/k + ↑/↓"), sepStyle.Render(": move"),
		sepStyle.Render("  |  "),
		keyStyle.Render("r"), sepStyle.Render(": REPL"),
		sepStyle.Render("  |  "),
		keyStyle.Render("Esc"), sepStyle.Render(": menu"),
		sepStyle.Render("  |  "),
		keyStyle.Render("q"), sepStyle.Render(": quit"),
	)
	rightLegend := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#4B8BBE")).Render("global"),
		lipgloss.NewStyle().Render(" / "),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#ffde57")).Render("venv"),
	)
	spacer := lipgloss.NewStyle().Width(max(0, m.view.Width-lipgloss.Width(leftLegend)-lipgloss.Width(rightLegend))).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, leftLegend, spacer, rightLegend)
}

func (m Model) renderREPLModal() string {
	selectedPath := "No interpreter selected"
	if len(m.interpreters) > 0 && m.selected < len(m.interpreters) {
		selectedPath = m.interpreters[m.selected].Path
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Render("REPL Launcher"),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(selectedPath),
		"",
		"Enter: open REPL",
		"Esc: cancel",
	}
	if m.replStatus != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(m.replStatus))
	}

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f2f2f2")).
		Padding(1, 2).
		Width(42).
		Render(strings.Join(lines, "\n"))

	return lipgloss.Place(m.view.Width, m.view.Height-1, lipgloss.Center, lipgloss.Center, modal)
}

// fillToHeight pads or trims lines to exactly height entries.
func fillToHeight(lines []string, height int) []string {
	if len(lines) >= height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func splitTwoWidths(total int) (int, int) {
	const gapWidth = 3
	available := total - gapWidth
	if available < 2 {
		return 0, 0
	}
	left := available / 2
	right := available - left
	return left, right
}

func panelStyle(focused bool) lipgloss.Style {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	if focused {
		return style.BorderForeground(lipgloss.Color(focusHighlightColor))
	}
	return style
}

func packageVisibleLines(panelInnerHeight, usedLines int) int {
	if v := panelInnerHeight - usedLines; v > 0 {
		return v
	}
	return 0
}

func detailsHeaderLines() int { return 10 }

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "Unknown"
	}
	return value
}
