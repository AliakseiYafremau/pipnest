package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"pipnest/internal/cheatsheet"
)

var stripTagsPattern = regexp.MustCompile(`<[^>]+>`)
var glowRenderCache = map[string]string{}
var ansiStripRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSIMain(s string) string { return ansiStripRe.ReplaceAllString(s, "") }

func overlayScrollbarOnBorder(box string, total, scroll, visibleRows int) string {
	if total <= visibleRows || visibleRows < 1 {
		return box
	}
	thumbHeight := visibleRows * visibleRows / total
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	maxScroll := total - visibleRows
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = scroll * (visibleRows - thumbHeight) / maxScroll
	}
	trackSt := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	thumbSt := lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Bold(true)
	lines := strings.Split(box, "\n")
	for i := 1; i < len(lines)-1; i++ {
		bodyIdx := i - 1
		var ch string
		if bodyIdx >= thumbPos && bodyIdx < thumbPos+thumbHeight {
			ch = thumbSt.Render("┃")
		} else {
			ch = trackSt.Render("│")
		}
		idx := strings.LastIndex(lines[i], "│")
		if idx >= 0 {
			lines[i] = lines[i][:idx] + ch + lines[i][idx+len("│"):]
		}
	}
	return strings.Join(lines, "\n")
}

func injectBorderTitle(box, title string, borderColor lipgloss.Color, focused bool) string {
	lines := strings.Split(box, "\n")
	if len(lines) == 0 {
		return box
	}
	plain := stripANSIMain(lines[0])
	width := lipgloss.Width(plain)
	if width < 4 {
		return box
	}
	titleStr := " " + title + " "
	titleW := lipgloss.Width(titleStr)
	inner := width - 2
	if titleW > inner-1 {
		return box
	}
	dashSt := lipgloss.NewStyle().Foreground(borderColor)
	if focused {
		dashSt = dashSt.Bold(true)
	}
	titleSt := lipgloss.NewStyle().Foreground(borderColor).Bold(true)
	remaining := inner - titleW
	lines[0] = dashSt.Render("╭─") + titleSt.Render(titleStr) + dashSt.Render(strings.Repeat("─", remaining-1)+"╮")
	return strings.Join(lines, "\n")
}

const (
	mainMenuMinWidth  = 60
	mainMenuMinHeight = 12
)

type mainMenuGeometry struct {
	menuWidth     int
	contentHeight int
	startX        int
	startY        int
	optionStartY  int
}

func computeMainMenuGeometry(m model) mainMenuGeometry {
	viewWidth := m.width
	if viewWidth < 30 {
		viewWidth = 30
	}
	viewHeight := m.height
	if viewHeight < 12 {
		viewHeight = 12
	}
	// Main menu content is rendered in the top area, reserving the last row for legend.
	contentAreaHeight := max(1, viewHeight-1)

	menuWidth := viewWidth - 8
	if menuWidth > 96 {
		menuWidth = 96
	}
	if menuWidth < 40 {
		menuWidth = 40
	}

	logoHeight := lipgloss.Height(cheatsheet.LogoTitle)
	if logoHeight < 1 {
		logoHeight = 1
	}
	// Center only the menu content. Legend is rendered separately on bottom line.
	contentHeight := logoHeight + 2 + len(MainMenuItems) + 2
	if contentHeight > contentAreaHeight {
		contentHeight = contentAreaHeight
	}

	startX := 0
	if viewWidth > menuWidth {
		startX = (viewWidth - menuWidth) / 2
	}
	startY := 0
	if contentAreaHeight > contentHeight {
		startY = (contentAreaHeight - contentHeight) / 2
	}

	return mainMenuGeometry{
		menuWidth:     menuWidth,
		contentHeight: contentHeight,
		startX:        startX,
		startY:        startY,
		optionStartY:  startY + logoHeight + 2,
	}
}

func renderMainMenuInsufficientSpace(m model) string {
	width := m.width
	if width < 1 {
		width = 1
	}
	height := m.height
	if height < 1 {
		height = 1
	}

	message := strings.Join([]string{
		"Not enough terminal space",
		fmt.Sprintf("Current: %dx%d", m.width, m.height),
		fmt.Sprintf("Minimum: %dx%d", mainMenuMinWidth, mainMenuMinHeight),
		"Resize the terminal to continue.",
	}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(message)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// renderMainMenu: Renderiza el menú principal
func renderMainMenu(m model) string {
	if m.width < mainMenuMinWidth || m.height < mainMenuMinHeight {
		return renderMainMenuInsufficientSpace(m)
	}

	geom := computeMainMenuGeometry(m)
	width := m.width
	if width < 30 {
		width = 30
	}
	height := m.height
	if height < 12 {
		height = 12
	}

	const (
		menuMutedColor  = "8"
		menuTitleColor  = "5"
		menuValueColor  = "4"
		menuAccentColor = "2"
	)

	menuItemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(lipgloss.Color(menuValueColor)).
		Width(geom.menuWidth - 4)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Bold(true).
		Reverse(true).
		Width(geom.menuWidth - 4)

	var lines []string

	// Agregar ASCII art logo
	logoStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(menuTitleColor)).
		Width(geom.menuWidth).
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(menuAccentColor)).
		Width(geom.menuWidth).
		Align(lipgloss.Center)

	// hintStyle := lipgloss.NewStyle().
	// 	Foreground(lipgloss.Color(menuMutedColor)).
	// 	Width(geom.menuWidth).
	// 	Align(lipgloss.Center)

	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(menuAccentColor))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(menuMutedColor))
	legendLeft := lipgloss.JoinHorizontal(lipgloss.Top,
		keyStyle.Render("Enter"), sepStyle.Render(": select"),
		sepStyle.Render("  |  "),
		keyStyle.Render("j/k + ↑/↓"), sepStyle.Render(": move"),
		sepStyle.Render("  |  "),
		keyStyle.Render("R/V/C"), sepStyle.Render(": quick open"),
		sepStyle.Render("  |  "),
		keyStyle.Render("click"), sepStyle.Render(": select"),
		sepStyle.Render("  |  "),
		keyStyle.Render("q"), sepStyle.Render(": quit"),
	)
	legendRight := lipgloss.NewStyle().Foreground(lipgloss.Color(menuMutedColor)).Render("main menu")
	legendSpacer := lipgloss.NewStyle().Width(max(0, width-lipgloss.Width(legendLeft)-lipgloss.Width(legendRight))).Render("")
	legend := lipgloss.JoinHorizontal(lipgloss.Top, legendLeft, legendSpacer, legendRight)

	lines = append(lines, logoStyle.Render(cheatsheet.LogoTitle))
	lines = append(lines, sepStyle.Render("\n\n"))
	lines = append(lines, titleStyle.Render("Select an option"))
	lines = append(lines, "")

	for i, item := range MainMenuItems {
		prefix := "  "
		if i == m.menuCursor {
			prefix = "> "
			lines = append(lines, selectedStyle.Render(prefix+item.Label))
		} else {
			lines = append(lines, menuItemStyle.Render(prefix+item.Label))
		}
	}

	content := strings.Join(lines, "\n")
	centered := lipgloss.Place(width, max(1, height-1), lipgloss.Center, lipgloss.Center, content)
	return strings.TrimRight(centered, "\n") + "\n" + legend
}

func renderResults(results []searchResult, width int, selectedIndex int) string {
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

func formatResultLine(result searchResult, width int) string {
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
		line += metaStyle.Render(" - " + truncateText(strings.TrimSpace(result.Description), summaryWidth))
	}

	if lipgloss.Width(line) > width {
		line = truncateText(line, width)
	}
	return line
}

func selectedSearchResult(results []searchResult, index int) *searchResult {
	if index < 0 || index >= len(results) {
		return nil
	}
	return &results[index]
}

func renderPackageDetails(result *searchResult, width int, loading bool, query string, err error) string {
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
		lines = append(lines, wrapText(err.Error(), width))
		return strings.Join(lines, "\n")
	}

	if loading && result == nil {
		lines = append(lines, metaStyle.Render("Loading results..."))
		return strings.Join(lines, "\n")
	}

	if result == nil {
		lines = append(lines, metaStyle.Render("Select a package on the left."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, valueStyle.Render(result.Name))
	if result.Version != "" {
		lines = append(lines, metaStyle.Render("Version"))
		lines = append(lines, wrapText(result.Version, width))
	}
	if result.Description != "" {
		lines = append(lines, metaStyle.Render("Summary"))
		lines = append(lines, wrapText(result.Description, width))
	}
	if result.URL != "" {
		lines = append(lines, metaStyle.Render("Project URL"))
		lines = append(lines, wrapText(result.URL, width))
	}

	return strings.Join(lines, "\n")
}

func truncateText(text string, max int) string {
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

func wrapText(text string, width int) string {
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

// renderPackagesScreen: Renderiza la pantalla de búsqueda de paquetes
func renderPackagesScreen(m model) string {
	if m.width == 0 {
		return ""
	}

	inputHeight := topInputHeight
	contentHeight := m.height - inputHeight - 1
	if contentHeight < 4 {
		contentHeight = 4
	}
	if contentHeight < 10 {
		contentHeight = 10
	}

	// Con RoundedBorder, Width(N) produce N+2 columnas reales (border izq + content + border der).
	// El separador "│" ocupa 1 columna.
	// Total: (left+2) + 1 + (right+2) = m.width  =>  left+right = m.width-5
	leftPaneWidth := (m.width - 5) / 2
	if leftPaneWidth < 24 {
		leftPaneWidth = 24
	}
	rightPaneWidth := m.width - 5 - leftPaneWidth
	if rightPaneWidth < 24 {
		rightPaneWidth = 24
	}

	// inputStyle: Width(N)+Border => N+2 cols. Para ocupar m.width exacto: N = m.width-2
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(m.width - 2).
		Height(inputHeight - 2)

	leftStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(leftPaneWidth).
		Height(contentHeight - 2)

	rightStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(rightPaneWidth).
		Height(contentHeight - 2)

	status := "Press Enter to search"
	if m.loading {
		status = "Searching..."
	} else if m.query != "" {
		status = fmt.Sprintf("Results for %q", m.query)
	}
	if m.err != nil {
		status = "Search error: " + m.err.Error()
	}

	inputBody := strings.Join([]string{m.input.View(), status}, "\n")
	resultsBody := renderResults(m.results, leftPaneWidth-4, m.selected)
	if resultsBody == "" {
		if m.loading {
			resultsBody = "Loading results..."
		} else {
			resultsBody = "Type a package name and press Enter."
		}
	}
	selectedResult := selectedSearchResult(m.results, m.selected)
	rightBody := renderPackageDetails(selectedResult, rightPaneWidth-4, m.loading, m.query, m.err)

	top := inputStyle.Render(inputBody)
	leftPane := leftStyle.Render(resultsBody)
	rightPane := rightStyle.Render(rightBody)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)

	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	footer := lipgloss.JoinHorizontal(lipgloss.Top,
		keyStyle.Render("Esc"), sepStyle.Render(": menu"),
		sepStyle.Render("  |  "),
		keyStyle.Render("←→"), sepStyle.Render(": switch pane"),
		sepStyle.Render("  |  "),
		keyStyle.Render("↑↓"), sepStyle.Render(": navigate/scroll"),
		sepStyle.Render("  |  "),
		keyStyle.Render("Ctrl+U/D"), sepStyle.Render(": scroll detail"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom, footer)
}

// renderRequirementsScreen: Renderiza la pantalla de requirements
func renderRequirementsScreen(m model) string {
	if m.width == 0 {
		return ""
	}

	body := m.requirements.View()
	if body == "" {
		return ""
	}

	return body
}

// renderVenvsScreen: Renderiza la pantalla de venvs
func renderVenvsScreen(m model) string {
	if m.venvsApp == nil {
		return ""
	}
	return m.venvsApp.View()
}

func renderCheatScreen(m model) string {
	if m.width == 0 {
		return ""
	}

	const accentColor = lipgloss.Color("5")
	const mutedColor = lipgloss.Color("8")

	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	snekStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("180"))

	// Search box
	searchBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(m.width - 2).
		Height(2)

	searchBox := searchBoxStyle.Render(m.cheatSearch.View())

	contentHeight := m.height - 5 // searchbox(4) + footer(1)
	if contentHeight < 8 {
		contentHeight = 8
	}

	listWidth := (m.width - 5) / 2
	detailsWidth := m.width - 5 - listWidth

	// inner height of the list pane (border takes 2 rows)
	visibleLines := contentHeight - 2
	if visibleLines < 1 {
		visibleLines = 1
	}
	// clamp scroll offset here so render and update stay in sync
	if m.cheatScrollOffset > len(m.filteredCommands)-visibleLines {
		m.cheatScrollOffset = max(0, len(m.filteredCommands)-visibleLines)
	}

	// focused pane styles
	basePaneStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	focusedStyle := basePaneStyle.Bold(true).BorderForeground(accentColor)

	listStyle := basePaneStyle.Width(listWidth).Height(contentHeight - 2)
	detailStyle := basePaneStyle.Width(detailsWidth).Height(contentHeight - 2)
	if m.cheatFocusedPane == 0 {
		listStyle = focusedStyle.Width(listWidth).Height(contentHeight - 2)
	} else {
		detailStyle = focusedStyle.Width(detailsWidth).Height(contentHeight - 2)
	}

	// — Left panel: command list —
	var cmdLines []string
	endIdx := m.cheatScrollOffset + visibleLines
	if endIdx > len(m.filteredCommands) {
		endIdx = len(m.filteredCommands)
	}

	// available inner width: listWidth (box content) - 2 (prefix) - 2 (border padding implicit)
	innerListWidth := listWidth - 2
	// split: cmd gets at most 60%, category gets at least 40% minus separator
	maxCmdCol := max(8, innerListWidth*6/10)
	minCatCol := max(6, innerListWidth*4/10-2)

	cmdColWidth := 8
	for i := m.cheatScrollOffset; i < endIdx; i++ {
		if w := lipgloss.Width(m.filteredCommands[i].Command); w > cmdColWidth {
			cmdColWidth = w
		}
	}
	if cmdColWidth > maxCmdCol {
		cmdColWidth = maxCmdCol
	}
	catColWidth := max(minCatCol, innerListWidth-cmdColWidth-2)

	for i := m.cheatScrollOffset; i < endIdx; i++ {
		cmd := m.filteredCommands[i]
		selected := i == m.cheatSelected

		cmdText := truncateText(cmd.Command, cmdColWidth)
		catText := truncateText(cmd.Category, catColWidth)

		cmdW := lipgloss.Width(cmdText)
		pad := strings.Repeat(" ", max(0, cmdColWidth-cmdW))
		row := strings.TrimRight(
			lipgloss.JoinHorizontal(lipgloss.Top,
				valueStyle.Render(cmdText+pad),
				"  ",
				metaStyle.Render(catText),
			), " ")
		if selected {
			cmdLines = append(cmdLines, lipgloss.NewStyle().Reverse(true).Bold(true).Render("> "+row))
		} else {
			cmdLines = append(cmdLines, "  "+row)
		}
	}

	cmdList := listStyle.Render(strings.Join(cmdLines, "\n"))
	if len(m.filteredCommands) > visibleLines {
		cmdList = overlayScrollbarOnBorder(cmdList, len(m.filteredCommands), m.cheatScrollOffset, visibleLines)
	}

	// — Right panel: detail + snek —
	var detailLines []string
	if m.cheatSelected >= 0 && m.cheatSelected < len(m.filteredCommands) {
		cmd := m.filteredCommands[m.cheatSelected]
		detailLines = append(detailLines,
			keyStyle.Render("Category"), cmd.Category,
			"",
			keyStyle.Render("Command"), wrapText(cmd.Command, detailsWidth-4),
			"",
			keyStyle.Render("Description"), wrapText(cmd.Description, detailsWidth-4),
		)
	} else {
		detailLines = append(detailLines, metaStyle.Render("No command selected."))
	}

	// snek art: solo se muestra si cabe completo (ancho y alto)
	snekLines := strings.Split(strings.TrimSpace(cheatsheet.SnekArt), "\n")
	panelInner := detailsWidth - 4
	totalDetailRows := contentHeight - 2
	availableSnekRows := totalDetailRows - len(detailLines) - 1
	snekMaxWidth := 0
	for _, sl := range snekLines {
		if w := lipgloss.Width(sl); w > snekMaxWidth {
			snekMaxWidth = w
		}
	}
	if availableSnekRows >= len(snekLines) && snekMaxWidth <= panelInner {
		detailLines = append(detailLines, "")
		for _, sl := range snekLines {
			lw := lipgloss.Width(sl)
			if lw == 0 {
				detailLines = append(detailLines, "")
				continue
			}
			sl = strings.Repeat(" ", (panelInner-lw)/2) + sl
			detailLines = append(detailLines, snekStyle.Render(sl))
		}
	}

	// scrollable detail
	flat := make([]string, 0, len(detailLines))
	for _, l := range detailLines {
		flat = append(flat, strings.Split(l, "\n")...)
	}
	maxScroll := max(0, len(flat)-totalDetailRows)
	if m.cheatDetailScroll > maxScroll {
		m.cheatDetailScroll = maxScroll
	}
	start := m.cheatDetailScroll
	end := start + totalDetailRows
	if end > len(flat) {
		end = len(flat)
	}
	visible := flat[start:end]
	for len(visible) < totalDetailRows {
		visible = append(visible, "")
	}

	detailPane := detailStyle.Render(strings.Join(visible, "\n"))
	if maxScroll > 0 {
		detailPane = overlayScrollbarOnBorder(detailPane, len(flat), start, totalDetailRows)
	}

	middleRow := lipgloss.JoinHorizontal(lipgloss.Top, cmdList, " ", detailPane)

	cheatKeyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	cheatSepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	footer := lipgloss.JoinHorizontal(lipgloss.Top,
		cheatKeyStyle.Render("Esc"), cheatSepStyle.Render(": menu"),
		cheatSepStyle.Render("  |  "),
		cheatKeyStyle.Render("←→"), cheatSepStyle.Render(": switch pane"),
		cheatSepStyle.Render("  |  "),
		cheatKeyStyle.Render("↑↓"), cheatSepStyle.Render(": navigate/scroll"),
		cheatSepStyle.Render("  |  "),
		cheatKeyStyle.Render("Enter"), cheatSepStyle.Render(": copy"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, searchBox, middleRow, footer)
}

func renderEasterEgg(m model) string {
	if m.width < 30 {
		m.width = 30
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	artStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Width(m.width - 4).Align(lipgloss.Center)
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("ESC to return to menu")

	body := strings.TrimSpace(cheatsheet.Macarrones)
	artBox := artStyle.Render(body)
	frame := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).Width(m.width - 2).Render(artBox)

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("🍝 Macarrones"),
		"",
		frame,
		"",
		footer,
	)
}
