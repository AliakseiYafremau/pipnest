package venvs

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var venvAnsiStripRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

var (
	mutedColor      = lipgloss.Color("8")
	globalKindColor = lipgloss.Color("6")
	venvKindColor   = lipgloss.Color("3")
	uiTitleColor    = lipgloss.Color("5")
	uiValueColor    = lipgloss.Color("4")
	uiKeyColor      = lipgloss.Color("2")
	uiVersionColor  = lipgloss.Color("1")
)

func (m *Model) View() string {
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

	row := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	ui := lipgloss.Place(m.view.Width, bodyHeight, lipgloss.Center, lipgloss.Top, row)

	if m.addModalOpen {
		ui = m.renderAddInterpreterModal()
	} else if m.replModalOpen {
		ui = m.renderREPLModal()
	}
	if m.runFileModalOpen {
		ui = m.renderRunFileModal()
	}
	if m.keybindsModalOpen {
		ui = m.renderKeybindsModal()
	}
	return strings.TrimRight(ui, "\n") + "\n" + legend
}

func (m *Model) renderInsufficientSpace() string {
	message := strings.Join([]string{
		"Not enough terminal space",
		fmt.Sprintf("Current: %dx%d", m.view.Width, m.view.Height),
		fmt.Sprintf("Minimum: %dx%d", minWidth, minHeight),
		"Resize the terminal to continue.",
	}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(message)

	return lipgloss.Place(m.view.Width, m.view.Height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) renderLeftPanel(width, height int) string {
	focused := !m.focusPackages
	innerHeight := max(1, height-4)
	maxWidth := max(1, width-4)

	muted := lipgloss.NewStyle().Foreground(mutedColor)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(uiTitleColor)

	lines := make([]string, 0, innerHeight)
	lines = append(lines, titleStyle.Render(truncateLine("Select interpreter", max(1, width-4))))
	lines = append(lines, "")

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

	rowTextWidth := maxWidth
	if len(m.interpreters) > availableRows && availableRows > 0 {
		rowTextWidth = max(1, rowTextWidth-2)
	}

	// compute label column width from visible slice
	const kindBadgeWidth = 7 // "[venv] " etc.
	const prefixWidth = 2    // "> " / "  "
	labelColWidth := 8
	for i := start; i < end; i++ {
		if w := lipgloss.Width(m.interpreters[i].Label); w > labelColWidth {
			labelColWidth = w
		}
	}
	maxLabelCol := max(8, (rowTextWidth-prefixWidth-kindBadgeWidth)/2)
	if labelColWidth > maxLabelCol {
		labelColWidth = maxLabelCol
	}

	rows := make([]string, 0, max(0, end-start))
	for i := start; i < end; i++ {
		option := m.interpreters[i]
		isActive := option.Path != "" && option.Path == m.view.Interpreter
		isSelected := i == m.selected && focused

		rows = append(rows, m.renderInterpreterRow(option, isSelected, isActive, labelColWidth, rowTextWidth))
	}

	if len(m.interpreters) > availableRows && availableRows > 0 {
		trackStyle := lipgloss.NewStyle().Foreground(mutedColor)
		thumbStyle := lipgloss.NewStyle().Foreground(uiTitleColor)
		rows = addScrollbar(rows, availableRows, len(m.interpreters), start, rowTextWidth+2, trackStyle, thumbStyle)
	}

	if len(m.interpreters) == 0 {
		lines = append(lines, muted.Render(truncateLine("No interpreters found.", max(1, width-4))))
	} else {
		lines = append(lines, rows...)
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

	muted := lipgloss.NewStyle().Foreground(mutedColor)
	kindAccent := accentForKind(kind)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(uiTitleColor)
	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(kindAccent)
	kindStyle := lipgloss.NewStyle().Bold(true).Foreground(kindAccent)
	versionStyle := lipgloss.NewStyle().Foreground(uiVersionColor)
	nameStyle := lipgloss.NewStyle().Foreground(uiValueColor)
	lines := []string{
		kindStyle.Render(truncateLine("Type: "+kindLabel(kind), maxW)),
		pathStyle.Render(truncateLine(path, maxW)),
		versionStyle.Render(truncateLine("Version: "+valueOrUnknown(details.Version), maxW)),
		muted.Render(truncateLine("Size: "+valueOrUnknown(details.SizeLabel), maxW)),
		muted.Render(truncateLine("Created: "+valueOrUnknown(details.CreatedAtLabel), maxW)),
		muted.Render(truncateLine("Updated: "+valueOrUnknown(details.UpdatedAtLabel), maxW)),
		muted.Render(truncateLine(fmt.Sprintf("Packages: %d", details.PackageCount), maxW)),
		muted.Render(truncateLine("Cmd: "+valueOrUnknown(details.ActivationCommand), maxW)),
		"",
		titleStyle.Render(truncateLine("Installed packages", maxW)),
	}

	// Show loading state if details are being fetched
	if m.loadingPath == path && len(details.Packages) == 0 && details.Version == "" {
		if len(lines) < innerHeight {
			lines = append(lines, kindStyle.Render(strings.TrimSpace(m.addSpinner.View()+" Loading...")))
		}
	} else if len(details.Packages) == 0 {
		if len(lines) < innerHeight {
			lines = append(lines, muted.Render("No packages found"))
		}
	} else {
		availableRows := packageVisibleLines(innerHeight, len(lines))
		displayRows := availableRows
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
		nameWidth := packageNameColumnWidth(details.Packages, m.packageScroll, end, maxW)
		pkgMaxW := max(1, maxW-2)
		if len(details.Packages) > displayRows && displayRows > 0 {
			pkgMaxW = max(1, pkgMaxW-2)
		}
		rows := make([]string, 0, max(0, end-m.packageScroll))
		for i := m.packageScroll; i < end && len(lines) < innerHeight; i++ {
			item := details.Packages[i]
			label := truncateLine(renderPackageRow(item, nameWidth, nameStyle, versionStyle), pkgMaxW)
			selected := i == m.packageSelected && m.focusPackages
			if selected {
				label = "> " + label
			} else {
				label = "  " + label
			}
			var lineStyle lipgloss.Style
			if selected {
				lineStyle = lipgloss.NewStyle().Reverse(true).Bold(true)
			} else {
				lineStyle = lipgloss.NewStyle()
			}
			rows = append(rows, lineStyle.Render(label))
		}
		if len(details.Packages) > displayRows && displayRows > 0 {
			trackStyle := lipgloss.NewStyle().Foreground(mutedColor)
			thumbStyle := lipgloss.NewStyle().Foreground(kindAccent)
			rows = addScrollbar(rows, displayRows, len(details.Packages), m.packageScroll, pkgMaxW+2, trackStyle, thumbStyle)
		}
		lines = append(lines, rows...)
	}

	lines = fillToHeight(lines, innerHeight)
	return panelStyle(m.focusPackages).
		Padding(1, 1).
		Width(width).
		Height(innerHeight).
		Render(strings.Join(lines, "\n"))
}

func (m *Model) renderLegend() string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(uiKeyColor)
	sepStyle := lipgloss.NewStyle().Foreground(mutedColor)
	separator := sepStyle.Render("  |  ")

	leftChunks := []string{
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("Enter"), sepStyle.Render(": select")),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("a"), sepStyle.Render(": add")),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("r"), sepStyle.Render(": REPL")),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("x"), sepStyle.Render(": run file")),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("Esc"), sepStyle.Render(": menu")),
		lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("q"), sepStyle.Render(": quit")),
	}
	rightLegend := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Foreground(accentForKind(InterpreterGlobal)).Render("global"),
		lipgloss.NewStyle().Render(" / "),
		lipgloss.NewStyle().Foreground(accentForKind(InterpreterVenv)).Render("venv"),
	)
	leftMax := max(1, m.view.Width-lipgloss.Width(rightLegend)-1)
	leftLegend := fitVenvLegendChunks(leftChunks, separator, leftMax, keyStyle, sepStyle)
	spacer := lipgloss.NewStyle().Width(max(0, m.view.Width-lipgloss.Width(leftLegend)-lipgloss.Width(rightLegend))).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, leftLegend, spacer, rightLegend)
}

func fitVenvLegendChunks(chunks []string, separator string, width int, keyStyle lipgloss.Style, sepStyle lipgloss.Style) string {
	if width <= 0 || len(chunks) == 0 {
		return ""
	}

	included := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		candidate := strings.Join(append(append([]string{}, included...), chunk), separator)
		if lipgloss.Width(venvAnsiStripRe.ReplaceAllString(candidate, "")) <= width {
			included = append(included, chunk)
			continue
		}
		break
	}

	hidden := len(chunks) - len(included)
	if hidden <= 0 {
		return strings.Join(included, separator)
	}

	for {
		overflowChunk := lipgloss.JoinHorizontal(lipgloss.Top, keyStyle.Render("?"), sepStyle.Render(fmt.Sprintf(": +%d", hidden)))
		candidateParts := append(append([]string{}, included...), overflowChunk)
		candidate := strings.Join(candidateParts, separator)
		if lipgloss.Width(venvAnsiStripRe.ReplaceAllString(candidate, "")) <= width {
			return candidate
		}
		if len(included) == 0 {
			plain := venvAnsiStripRe.ReplaceAllString(overflowChunk, "")
			return truncateLine(plain, width)
		}
		included = included[:len(included)-1]
		hidden++
	}
}

func (m *Model) renderREPLModal() string {
	selectedPath := "No interpreter selected"
	if len(m.interpreters) > 0 && m.selected < len(m.interpreters) {
		selectedPath = m.interpreters[m.selected].Path
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(uiTitleColor).Render("REPL Launcher"),
		lipgloss.NewStyle().Foreground(mutedColor).Render(selectedPath),
		"",
		"Enter: open REPL",
		"Esc: cancel",
	}
	if m.replStatus != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(mutedColor).Render(m.replStatus))
	}
	modalWidth := 42
	innerWidth := modalWidth - 6
	if innerWidth < 1 {
		innerWidth = 1
	}
	lines = normalizeVenvViewportLines(lines, innerWidth, len(lines))

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(modalWidth).
		Render(strings.Join(lines, "\n"))

	return lipgloss.Place(m.view.Width, m.view.Height-1, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) renderAddInterpreterModal() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(uiTitleColor)
	mutedStyle := lipgloss.NewStyle().Foreground(mutedColor)

	var lines []string
	if m.addCreating {
		spinnerStr := m.addSpinner.View()
		lines = []string{
			titleStyle.Render("Add Interpreter"),
			"",
			lipgloss.JoinHorizontal(lipgloss.Top, spinnerStr, " Creating environment…"),
			"",
			mutedStyle.Render("Please wait"),
		}
	} else {
		formView := ""
		if m.addForm != nil {
			formView = m.addForm.View()
		}
		helpText := "Enter: next/submit  Esc/q: cancel"
		if m.addLoading {
			helpText = "Working... please wait"
			if m.addStatus != "" {
				helpText = m.addStatus
			}
			formView = lipgloss.JoinHorizontal(
				lipgloss.Center,
				m.addSpinner.View(),
				" ",
				"Working on your interpreter setup",
			)
		}
		lines = []string{
			titleStyle.Render("Add Interpreter"),
			mutedStyle.Render(helpText),
			"",
			formView,
		}
		if m.addStatus != "" {
			lines = append(lines, "", mutedStyle.Render(m.addStatus))
		}
	}

	modalWidth := max(56, min(m.view.Width-8, 100))
	innerWidth := modalWidth - 6
	if innerWidth < 1 {
		innerWidth = 1
	}
	maxRows := max(1, m.view.Height-6)
	lines = normalizeVenvViewportLines(lines, innerWidth, maxRows)
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(modalWidth).
		Render(strings.Join(lines, "\n"))

	return lipgloss.Place(m.view.Width, m.view.Height-1, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) renderKeybindsModal() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(uiTitleColor).Render("Keybinds")
	muted := lipgloss.NewStyle().Foreground(mutedColor)
	key := lipgloss.NewStyle().Bold(true).Foreground(uiKeyColor)
	detail := lipgloss.NewStyle().Foreground(uiValueColor)

	rows := []string{
		title,
		muted.Render("Core"),
		key.Render("Enter") + detail.Render("  select / open interpreter list"),
		key.Render("j/k or ↑/↓") + detail.Render("  move in active list"),
		key.Render("Mouse left") + detail.Render("  click to open/select lists"),
		key.Render("←/→") + detail.Render("  switch focus between lists"),
		key.Render("Esc") + detail.Render("  close list focus / return to menu"),
		key.Render("q") + detail.Render("  quit app"),
		"",
		muted.Render("Secondary"),
		key.Render("r") + detail.Render("  open Python REPL using selected interpreter"),
		key.Render("x") + detail.Render("  run a Python file with selected interpreter"),
		key.Render("Mouse wheel") + detail.Render("  scroll package list"),
		"",
		muted.Render("Close help: Esc, or q"),
	}

	modalWidth := min(max(56, m.view.Width-12), 90)
	if modalWidth < 56 {
		modalWidth = 56
	}
	innerWidth := modalWidth - 6
	if innerWidth < 1 {
		innerWidth = 1
	}
	maxRows := max(1, m.view.Height-6)
	rows = normalizeVenvViewportLines(rows, innerWidth, maxRows)
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(modalWidth).
		Render(strings.Join(rows, "\n"))

	return lipgloss.Place(m.view.Width, m.view.Height-1, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) renderRunFileModal() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(uiTitleColor).Render("Run File")
	muted := lipgloss.NewStyle().Foreground(mutedColor)
	inputStyle := lipgloss.NewStyle().Foreground(uiValueColor)
	path := strings.TrimSpace(m.runFilePath)
	if path == "" {
		path = ""
	}
	cursorPath := path + "_"
	rows := []string{
		title,
		muted.Render("Enter a Python file path to execute with selected interpreter:"),
		inputStyle.Render(cursorPath),
		"",
		muted.Render("Enter: run   Esc: cancel"),
	}
	if m.runFileStatus != "" {
		rows = append(rows, "", muted.Render(m.runFileStatus))
	}
	modalWidth := min(max(60, m.view.Width-12), 100)
	innerWidth := modalWidth - 6
	if innerWidth < 1 {
		innerWidth = 1
	}
	maxRows := max(1, m.view.Height-6)
	rows = normalizeVenvViewportLines(rows, innerWidth, maxRows)
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(modalWidth).
		Render(strings.Join(rows, "\n"))
	return lipgloss.Place(m.view.Width, m.view.Height-1, lipgloss.Center, lipgloss.Center, modal)
}

func normalizeVenvViewportLines(lines []string, width int, rows int) []string {
	if width < 1 {
		width = 1
	}
	if rows < 1 {
		rows = 1
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		for _, part := range strings.Split(line, "\n") {
			out = append(out, clampVenvLineToWidth(part, width))
		}
	}
	if len(out) > rows {
		out = out[:rows]
	}
	return out
}

func clampVenvLineToWidth(line string, width int) string {
	if width < 1 {
		return ""
	}
	plain := venvAnsiStripRe.ReplaceAllString(line, "")
	if lipgloss.Width(plain) <= width {
		return line
	}
	return truncateLine(plain, width)
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
	const gapWidth = 0
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
		return style.Bold(true).BorderForeground(uiTitleColor)
	}
	return style
}

func accentForKind(kind InterpreterKind) lipgloss.TerminalColor {
	switch kind {
	case InterpreterVenv, InterpreterConda:
		return venvKindColor
	default:
		return globalKindColor
	}
}

func kindLabel(kind InterpreterKind) string {
	switch kind {
	case InterpreterVenv:
		return "Virtual environment"
	case InterpreterConda:
		return "Conda environment"
	default:
		return "Global interpreter"
	}
}

func packageVisibleLines(panelInnerHeight, usedLines int) int {
	if v := panelInnerHeight - usedLines; v > 0 {
		return v
	}
	return 0
}

func detailsHeaderLines() int { return 10 }

func kindBadge(kind InterpreterKind) string {
	switch kind {
	case InterpreterVenv:
		return "[venv]"
	case InterpreterConda:
		return "[conda]"
	default:
		return "[global]"
	}
}

func (m *Model) renderInterpreterRow(option InterpreterOption, isSelected, isActive bool, labelColWidth, rowTextWidth int) string {
	kindStyle := lipgloss.NewStyle().Foreground(accentForKind(option.Kind))
	mutedStyle := lipgloss.NewStyle().Foreground(mutedColor)

	label := option.Label
	labelW := lipgloss.Width(label)
	if labelW > labelColWidth {
		label = truncateLine(label, labelColWidth)
		labelW = labelColWidth
	}
	labelCol := kindStyle.Render(label + strings.Repeat(" ", max(0, labelColWidth-labelW)))

	pathAvail := rowTextWidth - 2 - labelColWidth - 2 // prefix + label + sep
	path := abbreviatePath(option.Path, max(0, pathAvail))
	pathCol := mutedStyle.Render(path)

	row := strings.TrimRight(lipgloss.JoinHorizontal(lipgloss.Top, labelCol, "  ", pathCol), " ")
	row = truncateLine(row, rowTextWidth-2)

	switch {
	case isSelected && isActive:
		return lipgloss.NewStyle().Reverse(true).Bold(true).Render("> " + row + " <")
	case isSelected:
		return lipgloss.NewStyle().Reverse(true).Bold(true).Render("> " + row)
	case isActive:
		return kindStyle.Bold(true).Render("» " + row + " «")
	default:
		return "  " + row
	}
}

// abbreviatePath shortens a path to fit within maxWidth.
// It replaces the home directory with ~, then if still too long,
// keeps as many trailing path components as fit with a …/ prefix.
func abbreviatePath(path string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	if lipgloss.Width(path) <= maxWidth {
		return path
	}
	parts := strings.Split(path, string(os.PathSeparator))
	for i := 1; i < len(parts); i++ {
		candidate := "…/" + strings.Join(parts[i:], string(os.PathSeparator))
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return truncateLine(path, maxWidth)
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

func packageNameColumnWidth(packages []PackageInfo, start, end, maxWidth int) int {
	if start < 0 {
		start = 0
	}
	if end > len(packages) {
		end = len(packages)
	}
	if end <= start {
		return max(8, maxWidth/2)
	}
	maxName := 0
	for i := start; i < end; i++ {
		w := lipgloss.Width(packages[i].Name)
		if w > maxName {
			maxName = w
		}
	}
	capWidth := max(8, maxWidth-14)
	if maxName > capWidth {
		return capWidth
	}
	if maxName < 8 {
		return 8
	}
	return maxName
}

func renderPackageRow(item PackageInfo, nameWidth int, nameStyle, versionStyle lipgloss.Style) string {
	name := item.Name
	if lipgloss.Width(name) > nameWidth {
		name = truncateLine(name, nameWidth)
	}
	padding := nameWidth - lipgloss.Width(name)
	if padding < 0 {
		padding = 0
	}
	nameCol := nameStyle.Render(name + strings.Repeat(" ", padding))
	version := versionStyle.Render(item.Version)
	return lipgloss.JoinHorizontal(lipgloss.Top, nameCol, "  ", version)
}

func addScrollbar(rows []string, viewport, total, offset, contentWidth int, trackStyle, thumbStyle lipgloss.Style) []string {
	if viewport <= 0 {
		return rows
	}
	out := make([]string, 0, viewport)
	if total <= viewport {
		for i := 0; i < viewport; i++ {
			if i < len(rows) {
				out = append(out, rows[i])
			} else {
				out = append(out, "")
			}
		}
		return out
	}
	thumbSize := max(1, (viewport*viewport)/total)
	if thumbSize > viewport {
		thumbSize = viewport
	}
	maxStart := max(0, viewport-thumbSize)
	thumbStart := 0
	if total > viewport && maxStart > 0 {
		thumbStart = (offset * maxStart) / (total - viewport)
	}

	for i := 0; i < viewport; i++ {
		base := ""
		if i < len(rows) {
			base = rows[i]
		}
		if contentWidth > 0 {
			pad := contentWidth - lipgloss.Width(base)
			if pad > 0 {
				base += strings.Repeat(" ", pad)
			}
		}
		glyph := trackStyle.Render("│")
		if i >= thumbStart && i < thumbStart+thumbSize {
			glyph = thumbStyle.Render("█")
		}
		out = append(out, base+"  "+glyph)
	}
	return out
}
