package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pipnest/internal/venvs"
)

type venvsModel struct {
	view              venvs.ViewModel
	interpreters      []venvs.InterpreterOption
	selected          int
	dropdownOpen      bool
	focusPackages     bool
	packageSelected   int
	packageScroll     int
	replModalOpen     bool
	replStatus        string
	activationCommand string
	activationMessage string
	startedWithVenv   bool
	detailsCache      map[string]venvs.InterpreterDetails
	highlighted       venvs.InterpreterDetails
}

const (
	focusHighlightColor = "57"
	minVenvsWidth       = 72
	minVenvsHeight      = 14
)

type replFinishedMsg struct {
	err error
}

func newVenvsModel() venvsModel {
	model := venvsModel{
		view:            venvs.NewViewModel(),
		interpreters:    venvs.ListInterpreters(),
		startedWithVenv: os.Getenv("VIRTUAL_ENV") != "",
		detailsCache:    make(map[string]venvs.InterpreterDetails),
	}
	model.applySelection()
	model.refreshHighlightedDetails()
	return model
}

func (m venvsModel) Init() tea.Cmd { return nil }

func (m venvsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.replModalOpen {
			if msg.Type == tea.KeyEsc {
				m.replModalOpen = false
				m.replStatus = ""
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				if len(m.interpreters) == 0 || m.selected >= len(m.interpreters) {
					m.replModalOpen = false
					m.replStatus = ""
					return m, nil
				}
				replPath := m.interpreters[m.selected].Path
				if replPath == "" {
					m.replModalOpen = false
					m.replStatus = ""
					return m, nil
				}
				m.replModalOpen = false
				m.replStatus = ""
				cmd := exec.Command(replPath)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					return replFinishedMsg{err: err}
				})
			}
			return m, nil
		}

		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			m.applySelection()
			return m, tea.Quit
		}
		if msg.Type == tea.KeyRight {
			if len(m.highlighted.Packages) > 0 {
				m.dropdownOpen = false
				m.focusPackages = true
			}
			return m, nil
		}
		if msg.Type == tea.KeyLeft {
			m.focusPackages = false
			m.dropdownOpen = true
			return m, nil
		}
		if msg.String() == "r" {
			if len(m.interpreters) > 0 {
				m.replModalOpen = true
				m.replStatus = "Open REPL with selected interpreter"
			}
			return m, nil
		}
		if msg.Type == tea.KeyEsc {
			if m.dropdownOpen {
				m.dropdownOpen = false
				m.focusPackages = false
				return m, nil
			}
			if m.focusPackages {
				m.focusPackages = false
				return m, nil
			}
			return m, tea.Quit
		}
		if msg.Type == tea.KeyEnter {
			if len(m.interpreters) == 0 {
				return m, nil
			}
			if m.focusPackages {
				m.focusPackages = false
				return m, nil
			}
			if !m.dropdownOpen {
				m.dropdownOpen = true
				m.refreshHighlightedDetails()
				return m, nil
			}
			m.applySelection()
			m.dropdownOpen = false
			m.refreshHighlightedDetails()
			return m, nil
		}
		if m.focusPackages {
			switch msg.String() {
			case "k":
				m.scrollPackages(-1)
				return m, nil
			case "j":
				m.scrollPackages(1)
				return m, nil
			}
			switch msg.Type {
			case tea.KeyUp, tea.KeyCtrlP:
				m.scrollPackages(-1)
				return m, nil
			case tea.KeyDown, tea.KeyCtrlN:
				m.scrollPackages(1)
				return m, nil
			}
		}
		if m.dropdownOpen {
			switch msg.String() {
			case "k":
				if m.selected > 0 {
					m.selected--
					m.refreshHighlightedDetails()
				}
				return m, nil
			case "j":
				if m.selected < len(m.interpreters)-1 {
					m.selected++
					m.refreshHighlightedDetails()
				}
				return m, nil
			}
			switch msg.Type {
			case tea.KeyUp, tea.KeyCtrlP:
				if m.selected > 0 {
					m.selected--
					m.refreshHighlightedDetails()
				}
				return m, nil
			case tea.KeyDown, tea.KeyCtrlN:
				if m.selected < len(m.interpreters)-1 {
					m.selected++
					m.refreshHighlightedDetails()
				}
				return m, nil
			}
		}
	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.scrollPackages(-1)
			return m, nil
		case tea.MouseWheelDown:
			m.scrollPackages(1)
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.view.Width = msg.Width
		m.view.Height = msg.Height
	case replFinishedMsg:
		if msg.err != nil {
			m.replStatus = "REPL exited with error: " + msg.err.Error()
		} else {
			m.replStatus = "REPL closed"
		}
	}

	return m, nil
}

func (m venvsModel) View() string {
	if m.view.Width <= 0 || m.view.Height <= 0 {
		return ""
	}
	if m.view.Width < minVenvsWidth || m.view.Height < minVenvsHeight {
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
	centeredRow := lipgloss.Place(m.view.Width, bodyHeight, lipgloss.Center, lipgloss.Top, row)
	ui := centeredRow
	if m.replModalOpen {
		ui = overlay(ui, m.renderREPLModal())
	}
	return ui + "\n" + legend
}

func (m venvsModel) renderInsufficientSpace() string {
	message := []string{
		"Not enough terminal space",
		fmt.Sprintf("Current: %dx%d", m.view.Width, m.view.Height),
		fmt.Sprintf("Minimum: %dx%d", minVenvsWidth, minVenvsHeight),
		"Resize the terminal to continue.",
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#999999")).
		Padding(1, 2).
		Render(strings.Join(message, "\n"))

	return lipgloss.Place(m.view.Width, m.view.Height, lipgloss.Center, lipgloss.Center, box)
}

func fillHeight(lines []string, height int) []string {
	if len(lines) >= height {
		return lines[:height]
	}
	missing := height - len(lines)
	for i := 0; i < missing; i++ {
		lines = append(lines, "")
	}
	return lines
}

func (m *venvsModel) applySelection() {
	if len(m.interpreters) == 0 {
		m.activationCommand = ""
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.interpreters) {
		m.selected = len(m.interpreters) - 1
	}

	selected := m.interpreters[m.selected]
	if selected.Path == "" {
		m.activationCommand = ""
		m.activationMessage = ""
		return
	}

	m.view.Interpreter = selected.Path
	m.view.InterpreterKind = selected.Kind
	m.activationCommand = selected.ActivationCommand()
	m.activationMessage = "Activation command copied to clipboard. Paste and run it in your shell."
	if selected.Kind == venvs.InterpreterGlobal && m.startedWithVenv {
		m.activationCommand = fmt.Sprintf("deactivate # switched to global interpreter: %s", selected.Path)
		m.activationMessage = "Detected active venv at launch and selected global interpreter. Copied 'deactivate' command to clipboard."
	}
}

func (m *venvsModel) refreshHighlightedDetails() {
	if len(m.interpreters) == 0 {
		m.highlighted = venvs.InterpreterDetails{}
		m.packageSelected = 0
		m.packageScroll = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.interpreters) {
		m.selected = len(m.interpreters) - 1
	}
	m.highlighted = m.loadDetails(m.interpreters[m.selected])
	if m.packageSelected >= len(m.highlighted.Packages) {
		m.packageSelected = 0
	}
	m.packageScroll = 0
}

func (m *venvsModel) loadDetails(option venvs.InterpreterOption) venvs.InterpreterDetails {
	if option.Path == "" {
		return venvs.InterpreterDetails{}
	}
	if details, exists := m.detailsCache[option.Path]; exists {
		return details
	}
	details := option.Details()
	m.detailsCache[option.Path] = details
	return details
}

func (m venvsModel) ActivationCommand() string { return m.activationCommand }

func (m venvsModel) ActivationMessage() string { return m.activationMessage }

func (m venvsModel) renderLeftPanel(width, height int) string {
	focused := !m.focusPackages
	innerHeight := max(1, height-4)
	maxW := max(1, width-4)

	currentLabel := "Current environment"
	currentValue := "No active interpreter"
	if m.view.Interpreter != "" {
		currentValue = filepath.Base(m.view.Interpreter)
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine(currentLabel, maxW)),
		styleForInterpreter(m.view.InterpreterKind).Render(truncateLegendLine(currentValue, maxW)),
		"",
		lipgloss.NewStyle().Bold(true).Render(truncateLegendLine("Interpreter dropdown", maxW)),
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
		for index := start; index < end; index++ {
			option := m.interpreters[index]
			label := option.Label
			if option.Path != "" {
				label = fmt.Sprintf("%s - %s", option.Label, option.Path)
			}
			label = truncateLegendLine(label, maxWidth)
			lineStyle := lipgloss.NewStyle()
			if index == m.selected && focused {
				lineStyle = lineStyle.Background(lipgloss.Color(focusHighlightColor)).Foreground(lipgloss.Color("230"))
				label = "> " + label
			} else {
				label = "  " + label
			}
			lines = append(lines, lineStyle.Render(label))
		}
	} else {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Press Enter to open", maxW)))
	}
	lines = fillHeight(lines, innerHeight)
	return panelStyle(focused).
		Padding(1, 1).
		Width(width).
		Height(innerHeight).
		Render(strings.Join(lines, "\n"))
}

func (m *venvsModel) renderDetailsAndPackagesPanel(width, height int) string {
	details := m.highlighted
	innerHeight := max(1, height-4)
	maxW := max(1, width-4)

	kind := details.Kind
	if kind == "" {
		kind = venvs.InterpreterGlobal
	}

	path := details.Path
	if path == "" {
		path = "No interpreter selected"
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Highlighted interpreter", maxW)),
		styleForInterpreter(kind).Render(truncateLegendLine(path, maxW)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Version: "+valueOrUnknown(details.Version), maxW)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Size: "+valueOrUnknown(details.SizeLabel), maxW)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Created: "+valueOrUnknown(details.CreatedAtLabel), maxW)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Updated: "+valueOrUnknown(details.UpdatedAtLabel), maxW)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Packages: "+fmt.Sprintf("%d", details.PackageCount), maxW)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Cmd: "+valueOrUnknown(details.ActivationCommand), maxW)),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(truncateLegendLine("Installed packages", maxW)),
	}

	if len(details.Packages) == 0 {
		if len(lines) < innerHeight {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render("No packages found"))
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
		pkgMaxW := max(1, maxW-2) // account for "> " / "  " prefix
		for index := m.packageScroll; index < end && len(lines) < innerHeight; index++ {
			item := details.Packages[index]
			label := truncateLegendLine(fmt.Sprintf("%s %s", item.Name, item.Version), pkgMaxW)
			if index == m.packageSelected && m.focusPackages {
				label = "> " + label
			} else {
				label = "  " + label
			}
			lineStyle := lipgloss.NewStyle()
			if index == m.packageSelected && m.focusPackages {
				lineStyle = lineStyle.Background(lipgloss.Color(focusHighlightColor)).Foreground(lipgloss.Color("230"))
			}
			lines = append(lines, lineStyle.Render(label))
		}
		if remaining := len(details.Packages) - end; remaining > 0 {
			if len(lines) < innerHeight {
				lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render(fmt.Sprintf("+%d more", remaining)))
			}
		}
	}

	lines = fillHeight(lines, innerHeight)
	return panelStyle(m.focusPackages).
		Padding(1, 1).
		Width(width).
		Height(innerHeight).
		Render(strings.Join(lines, "\n"))
}

func (m venvsModel) renderLegend() string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f2f2f2"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	leftLegend := lipgloss.JoinHorizontal(
		lipgloss.Top,
		keyStyle.Render("Enter"), sepStyle.Render(": select"),
		sepStyle.Render("  |  "),
		keyStyle.Render("←/→"), sepStyle.Render(": focus"),
		sepStyle.Render("  |  "),
		keyStyle.Render("j/k + ↑/↓"), sepStyle.Render(": move"),
		sepStyle.Render("  |  "),
		keyStyle.Render("r"), sepStyle.Render(": REPL"),
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

func (m *venvsModel) scrollPackages(delta int) {
	if len(m.highlighted.Packages) == 0 {
		return
	}
	m.focusPackages = true
	m.packageSelected += delta
	if m.packageSelected < 0 {
		m.packageSelected = 0
	}
	if m.packageSelected >= len(m.highlighted.Packages) {
		m.packageSelected = len(m.highlighted.Packages) - 1
	}
	bodyHeight := m.view.Height - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	panelHeight := bodyHeight - 2
	if panelHeight < 1 {
		panelHeight = 1
	}
	visible := packageVisibleLines(max(1, panelHeight-4), detailsHeaderLines())
	if visible > 1 {
		visible--
	}
	if visible < 1 {
		visible = 1
	}
	if m.packageSelected < m.packageScroll {
		m.packageScroll = m.packageSelected
	}
	if m.packageSelected >= m.packageScroll+visible {
		m.packageScroll = m.packageSelected - visible + 1
	}
}

func interpreterColor(kind venvs.InterpreterKind) lipgloss.Color {
	switch kind {
	case venvs.InterpreterVenv, venvs.InterpreterConda:
		return lipgloss.Color("#ffde57")
	default:
		return lipgloss.Color("#4B8BBE")
	}
}

func truncateLegendLine(value string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= maxWidth {
		return value
	}
	ellipsis := "..."
	if maxWidth <= len(ellipsis) {
		return strings.Repeat(".", maxWidth)
	}
	remaining := maxWidth - len(ellipsis)
	runes := []rune(value)
	if remaining >= len(runes) {
		return value
	}
	left := remaining / 2
	right := remaining - left
	return string(runes[:left]) + ellipsis + string(runes[len(runes)-right:])
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func packageVisibleLines(panelInnerHeight int, usedLines int) int {
	visible := panelInnerHeight - usedLines
	if visible < 0 {
		return 0
	}
	return visible
}

func detailsHeaderLines() int {
	return 10
}

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

func styleForInterpreter(kind venvs.InterpreterKind) lipgloss.Style {
	switch kind {
	case venvs.InterpreterVenv, venvs.InterpreterConda:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffde57"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4B8BBE"))
	}
}

func (m venvsModel) renderREPLModal() string {
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

func overlay(base string, top string) string {
	if top == "" {
		return base
	}
	return top
}
