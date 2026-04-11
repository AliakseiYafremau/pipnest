package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
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
	packagesMinWidth  = 56
	packagesMinHeight = 12
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

func renderPackagesInsufficientSpace(m model) string {
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
		fmt.Sprintf("Minimum: %dx%d", packagesMinWidth, packagesMinHeight),
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
		keyStyle.Render("R/I/C"), sepStyle.Render(": quick open"),
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

func renderResults(results []searchResult, width int, selectedIndex int, scroll int, visibleRows int) string {
	if width < 20 {
		width = 20
	}

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("57"))

	headerStyle := lipgloss.NewStyle().Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	if len(results) == 0 {
		return strings.Join([]string{
			headerStyle.Render("Packages"),
			subtitleStyle.Render("←/→ focus  ↑/↓ navigate"),
			"",
			"Type a package name and press Enter.",
		}, "\n")
	}

	// clamp scroll so selected is visible
	if selectedIndex < scroll {
		scroll = selectedIndex
	}
	if selectedIndex >= scroll+visibleRows {
		scroll = selectedIndex - visibleRows + 1
	}
	if scroll < 0 {
		scroll = 0
	}

	contentWidth := width - 1 // 1 col for scrollbar
	if contentWidth < 10 {
		contentWidth = 10
	}

	var lines []string
	lines = append(lines, headerStyle.Render("Packages"))
	lines = append(lines, subtitleStyle.Render("←/→ focus  ↑/↓ navigate"))
	lines = append(lines, "")

	end := scroll + visibleRows
	if end > len(results) {
		end = len(results)
	}
	for i := scroll; i < end; i++ {
		line := formatResultLine(results[i], contentWidth)
		if i == selectedIndex {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}

	// attach scrollbar
	scrollbar := renderScrollbar(len(results), scroll, visibleRows)
	return attachScrollbar(strings.Join(lines, "\n"), scrollbar, 3, contentWidth)
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

func renderPackageDetails(result *searchResult, width int, height int, scroll int, loading bool, query string, err error) (string, int) {
	if width < 24 {
		width = 24
	}
	if height < 4 {
		height = 4
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
		return joinScrollableLines(lines, width, height, scroll)
	}

	if loading && result == nil {
		lines = append(lines, metaStyle.Render("Loading results..."))
		return joinScrollableLines(lines, width, height, scroll)
	}

	if result == nil {
		lines = append(lines, metaStyle.Render("Select a package on the left."))
		return joinScrollableLines(lines, width, height, scroll)
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
	if strings.TrimSpace(result.Readme) != "" {
		lines = append(lines, "")
		lines = append(lines, metaStyle.Render("README"))
		lines = append(lines, renderMarkdownWithGlow(result.Readme, width))
	}
	if result.URL != "" {
		lines = append(lines, "")
		lines = append(lines, metaStyle.Render("Project URL"))
		lines = append(lines, wrapText(result.URL, width))
	}

	return joinScrollableLines(lines, width, height, scroll)
}

func joinScrollableLines(lines []string, width int, height int, scroll int) (string, int) {
	flat := make([]string, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, "\n")
		flat = append(flat, parts...)
	}
	for i := range flat {
		flat[i] = clampMainLineToWidth(flat[i], width)
	}

	maxScroll := 0
	if len(flat) > height {
		maxScroll = len(flat) - height
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	visible := height
	if visible < 1 {
		visible = 1
	}
	start := scroll
	end := start + visible
	if end > len(flat) {
		end = len(flat)
	}
	chunk := append([]string{}, flat[start:end]...)
	for len(chunk) < visible {
		chunk = append(chunk, "")
	}

	if maxScroll > 0 {
		scrollbar := renderScrollbar(len(flat), scroll, visible)
		contentWidth := width - 1
		if contentWidth < 1 {
			contentWidth = 1
		}
		return attachScrollbar(strings.Join(chunk, "\n"), scrollbar, 0, contentWidth), maxScroll
	}
	return strings.Join(chunk, "\n"), 0
}

func clampMainLineToWidth(line string, width int) string {
	if width < 1 {
		return ""
	}
	plain := stripANSIMain(line)
	if lipgloss.Width(plain) <= width {
		return line
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

// renderScrollbar builds a vertical scrollbar string of `visibleRows` characters
// representing position `scroll` within `total` items.
func renderScrollbar(total, scroll, visibleRows int) string {
	if total <= visibleRows || visibleRows < 1 {
		return strings.Repeat(" ", visibleRows)
	}
	trackHeight := visibleRows
	thumbHeight := trackHeight * visibleRows / total
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	maxScroll := total - visibleRows
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = scroll * (trackHeight - thumbHeight) / maxScroll
	}

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	var sb strings.Builder
	for i := 0; i < trackHeight; i++ {
		if i >= thumbPos && i < thumbPos+thumbHeight {
			sb.WriteString(thumbStyle.Render("█"))
		} else {
			sb.WriteString(barStyle.Render("│"))
		}
		if i < trackHeight-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// attachScrollbar overlays a 1-col scrollbar on the right side of rendered content.
// headerRows is the number of lines at the top that should not get a scrollbar char.
// contentWidth is the width of the content (scrollbar appended after).
func attachScrollbar(content string, scrollbar string, headerRows int, contentWidth int) string {
	contentLines := strings.Split(content, "\n")
	barLines := strings.Split(scrollbar, "\n")

	result := make([]string, len(contentLines))
	barIdx := 0
	for i, line := range contentLines {
		if i < headerRows {
			result[i] = line
			continue
		}
		// pad/trim line to contentWidth
		w := lipgloss.Width(line)
		if w < contentWidth {
			line = line + strings.Repeat(" ", contentWidth-w)
		}
		bar := " "
		if barIdx < len(barLines) {
			bar = barLines[barIdx]
			barIdx++
		}
		result[i] = line + bar
	}
	return strings.Join(result, "\n")
}

func renderMarkdownWithGlow(markdown string, width int) string {
	md := strings.TrimSpace(markdown)
	if md == "" {
		return ""
	}
	if width < 20 {
		width = 20
	}

	h := sha1.Sum([]byte(md))
	cacheKey := fmt.Sprintf("%d:%x", width, h)
	if cached, ok := glowRenderCache[cacheKey]; ok {
		return cached
	}

	glowPath, err := exec.LookPath("glow")
	if err != nil {
		rendered := wrapMarkdownFallback(md, width)
		glowRenderCache[cacheKey] = rendered
		return rendered
	}

	cmd := exec.Command(glowPath, "-", "-w", strconv.Itoa(width))
	cmd.Stdin = strings.NewReader(md)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		rendered := wrapMarkdownFallback(md, width)
		glowRenderCache[cacheKey] = rendered
		return rendered
	}

	rendered := strings.TrimRight(out.String(), "\n")
	if strings.TrimSpace(rendered) == "" {
		rendered = wrapMarkdownFallback(md, width)
	}
	glowRenderCache[cacheKey] = rendered
	return rendered
}

func wrapMarkdownFallback(markdown string, width int) string {
	lines := strings.Split(markdown, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if strings.TrimSpace(trimmed) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, strings.Split(wrapText(trimmed, width), "\n")...)
	}
	return strings.Join(out, "\n")
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
	if m.width < packagesMinWidth || m.height < packagesMinHeight {
		return renderPackagesInsufficientSpace(m)
	}

	inputHeight := topInputHeight
	contentHeight := m.height - inputHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Con RoundedBorder, Width(N) produce N+2 columnas reales (border izq + content + border der).
	// El separador "│" ocupa 1 columna.
	// Total: (left+2) + 1 + (right+2) = m.width  =>  left+right = m.width-5
	leftPaneWidth := (m.width - 5) / 2
	rightPaneWidth := m.width - 5 - leftPaneWidth

	// inputStyle: Width(N)+Border => N+2 cols. Para ocupar m.width exacto: N = m.width-2
	focusColor := lipgloss.Color("12")
	unfocusColor := lipgloss.Color("7")
	inputBorderColor := lipgloss.Color("7")
	focusedPane := m.focusedPane
	if focusedPane != 1 {
		focusedPane = 0
	}
	leftBorderColor := unfocusColor
	rightBorderColor := unfocusColor
	if focusedPane == 0 {
		leftBorderColor = focusColor
	} else {
		rightBorderColor = focusColor
	}

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(inputBorderColor).
		Width(m.width - 2).
		Height(inputHeight - 2)

	leftStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(leftBorderColor).
		Width(leftPaneWidth).
		Height(contentHeight - 2)

	rightStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rightBorderColor).
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

	innerHeight := contentHeight - 4
	if innerHeight < 1 {
		innerHeight = 1
	}

	inputBody := strings.Join([]string{m.input.View(), status}, "\n")
	resultsBody := renderResults(m.results, leftPaneWidth-2, m.selected, m.listScroll, innerHeight)
	selectedResult := selectedSearchResult(m.results, m.selected)
	rightBody, maxDetailScroll := renderPackageDetails(selectedResult, rightPaneWidth-4, innerHeight, m.detailScroll, m.loading, m.query, m.err)
	if m.detailScroll > maxDetailScroll {
		m.detailScroll = maxDetailScroll
	}

	top := inputStyle.Render(inputBody)
	leftPane := leftStyle.Render(resultsBody)
	rightPane := rightStyle.Render(rightBody)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)

	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	footer := lipgloss.JoinHorizontal(lipgloss.Top,
		keyStyle.Render("←/→"), sepStyle.Render(": switch pane"),
		sepStyle.Render("  |  "),
		keyStyle.Render("↑/↓"), sepStyle.Render(": navigate/scroll"),
		sepStyle.Render("  |  "),
		keyStyle.Render("Ctrl+U/D"), sepStyle.Render(": scroll detail"),
		sepStyle.Render("  |  "),
		keyStyle.Render("Esc"), sepStyle.Render(": menu"),
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
	for i := range flat {
		flat[i] = clampMainLineToWidth(flat[i], max(1, detailsWidth-2))
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
		cheatKeyStyle.Render("Enter"), cheatSepStyle.Render(": copy"),
		cheatSepStyle.Render("  |  "),
		cheatKeyStyle.Render("←/→"), cheatSepStyle.Render(": switch pane"),
		cheatSepStyle.Render("  |  "),
		cheatKeyStyle.Render("↑/↓"), cheatSepStyle.Render(": navigate/scroll"),
		cheatSepStyle.Render("  |  "),
		cheatKeyStyle.Render("Esc"), cheatSepStyle.Render(": menu"),
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
