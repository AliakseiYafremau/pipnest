package requirements

import (
	"context"
	"fmt"
	packagemanager "pipnest/internal/requirements/package_manager"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logKind string

const (
	logInfo    logKind = "info"
	logSuccess logKind = "success"
	logError   logKind = "error"
	logLoading logKind = "loading"
)

type ViewModel struct {
	Width          int
	Height         int
	PackageManager packagemanager.PackageManager

	Packages []packagemanager.Dependency
	Selected int
	Scroll   int

	LoadingList bool
	BusyAction  bool

	ModalOpen               bool
	InstallInput            textinput.Model
	Suggestions             []packagemanager.Dependency
	SuggestionSelected      int
	SuggestionScroll        int
	ModalLoadingSuggestions bool
	ModalLastQuery          string

	LogText string
	LogKind logKind
}

type listLoadedMsg struct {
	Packages []packagemanager.Dependency
	Err      error
}

type uninstallDoneMsg struct {
	Name string
	Err  error
}

type searchSuggestionsDoneMsg struct {
	Query   string
	Results []packagemanager.Dependency
	Err     error
}

type installDoneMsg struct {
	Name string
	Err  error
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func NewViewModel() ViewModel {
	installInput := textinput.New()
	installInput.Placeholder = "Type package name..."

	return ViewModel{
		PackageManager: packagemanager.NewUVManager("uv"),
		InstallInput:   installInput,
		LogText:        "Loading installed packages...",
		LogKind:        logLoading,
	}
}

func (m ViewModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadInstalledCmd())
}

func (m ViewModel) Update(msg tea.Msg) (ViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.ensureMainSelectionVisible(m.visibleMainRows())
		m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
		return m, nil
	case tea.KeyMsg:
		if m.ModalOpen {
			return m.updateInstallModal(msg)
		}
		return m.updateMainWindow(msg)
	case listLoadedMsg:
		m.LoadingList = false
		if msg.Err != nil {
			m.Packages = nil
			m.Selected = 0
			m.Scroll = 0
			m.setLog(logError, "Failed to load installed packages: "+msg.Err.Error())
			return m, nil
		}

		m.Packages = msg.Packages
		if len(m.Packages) == 0 {
			m.Selected = 0
			m.Scroll = 0
		} else {
			if m.Selected >= len(m.Packages) {
				m.Selected = len(m.Packages) - 1
			}
			if m.Selected < 0 {
				m.Selected = 0
			}
			m.ensureMainSelectionVisible(m.visibleMainRows())
		}
		m.setLog(logSuccess, fmt.Sprintf("Installed packages loaded: %d", len(m.Packages)))
		return m, nil
	case uninstallDoneMsg:
		m.BusyAction = false
		if msg.Err != nil {
			m.setLog(logError, "Uninstall failed: "+msg.Err.Error())
			return m, nil
		}

		m.setLog(logSuccess, fmt.Sprintf("Uninstalled %s", msg.Name))
		m.LoadingList = true
		return m, m.loadInstalledCmd()
	case searchSuggestionsDoneMsg:
		if strings.TrimSpace(msg.Query) != strings.TrimSpace(m.ModalLastQuery) {
			return m, nil
		}

		m.ModalLoadingSuggestions = false
		if msg.Err != nil {
			m.Suggestions = nil
			m.SuggestionSelected = 0
			m.SuggestionScroll = 0
			m.setLog(logError, "Search failed: "+msg.Err.Error())
			return m, nil
		}

		m.Suggestions = msg.Results
		m.SuggestionSelected = 0
		m.SuggestionScroll = 0
		m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
		m.setLog(logInfo, fmt.Sprintf("Suggestions: %d", len(msg.Results)))
		return m, nil
	case installDoneMsg:
		m.BusyAction = false
		if msg.Err != nil {
			m.setLog(logError, "Install failed: "+msg.Err.Error())
			return m, nil
		}

		m.closeModal()
		m.setLog(logSuccess, fmt.Sprintf("Installed %s", msg.Name))
		m.LoadingList = true
		return m, m.loadInstalledCmd()
	}

	return m, nil
}

func (m ViewModel) updateMainWindow(msg tea.KeyMsg) (ViewModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlP:
		if m.Selected > 0 {
			m.Selected--
			m.ensureMainSelectionVisible(m.visibleMainRows())
		}
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if m.Selected < len(m.Packages)-1 {
			m.Selected++
			m.ensureMainSelectionVisible(m.visibleMainRows())
		}
		return m, nil
	case tea.KeyPgUp:
		step := m.visibleMainRows()
		if step < 1 {
			step = 1
		}
		m.Selected -= step
		if m.Selected < 0 {
			m.Selected = 0
		}
		m.ensureMainSelectionVisible(m.visibleMainRows())
		return m, nil
	case tea.KeyPgDown:
		step := m.visibleMainRows()
		if step < 1 {
			step = 1
		}
		m.Selected += step
		if m.Selected >= len(m.Packages) {
			m.Selected = len(m.Packages) - 1
			if m.Selected < 0 {
				m.Selected = 0
			}
		}
		m.ensureMainSelectionVisible(m.visibleMainRows())
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return m, nil
		}

		switch msg.Runes[0] {
		case 'i', 'I':
			m.openModal()
			m.setLog(logInfo, "Install mode opened")
			return m, nil
		case 'd', 'D':
			if m.BusyAction || m.LoadingList {
				return m, nil
			}
			if len(m.Packages) == 0 || m.Selected < 0 || m.Selected >= len(m.Packages) {
				m.setLog(logInfo, "No selected package to uninstall")
				return m, nil
			}
			name := strings.TrimSpace(m.Packages[m.Selected].Name)
			if name == "" {
				m.setLog(logInfo, "No selected package to uninstall")
				return m, nil
			}

			m.BusyAction = true
			m.setLog(logLoading, fmt.Sprintf("Uninstalling %s...", name))
			return m, m.uninstallCmd(name)
		case 'r', 'R':
			if m.BusyAction {
				return m, nil
			}
			m.LoadingList = true
			m.setLog(logLoading, "Refreshing installed packages...")
			return m, m.loadInstalledCmd()
		}
	}

	return m, nil
}

func (m ViewModel) updateInstallModal(msg tea.KeyMsg) (ViewModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.closeModal()
		m.setLog(logInfo, "Install mode closed")
		return m, nil
	case tea.KeyUp, tea.KeyCtrlP:
		if m.SuggestionSelected > 0 {
			m.SuggestionSelected--
			m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
		}
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if m.SuggestionSelected < len(m.Suggestions)-1 {
			m.SuggestionSelected++
			m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
		}
		return m, nil
	case tea.KeyPgUp:
		step := m.visibleSuggestionRows()
		if step < 1 {
			step = 1
		}
		m.SuggestionSelected -= step
		if m.SuggestionSelected < 0 {
			m.SuggestionSelected = 0
		}
		m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
		return m, nil
	case tea.KeyPgDown:
		step := m.visibleSuggestionRows()
		if step < 1 {
			step = 1
		}
		m.SuggestionSelected += step
		if m.SuggestionSelected >= len(m.Suggestions) {
			m.SuggestionSelected = len(m.Suggestions) - 1
			if m.SuggestionSelected < 0 {
				m.SuggestionSelected = 0
			}
		}
		m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && (msg.Runes[0] == 'i' || msg.Runes[0] == 'I') {
			if m.BusyAction {
				return m, nil
			}

			pkgName := strings.TrimSpace(m.InstallInput.Value())
			if len(m.Suggestions) > 0 && m.SuggestionSelected >= 0 && m.SuggestionSelected < len(m.Suggestions) {
				candidate := strings.TrimSpace(m.Suggestions[m.SuggestionSelected].Name)
				if candidate != "" {
					pkgName = candidate
				}
			}

			if pkgName == "" {
				m.setLog(logInfo, "Type package name or select suggestion")
				return m, nil
			}

			m.BusyAction = true
			m.setLog(logLoading, fmt.Sprintf("Installing %s...", pkgName))
			return m, m.installCmd(pkgName)
		}
	}

	before := m.InstallInput.Value()
	var inputCmd tea.Cmd
	m.InstallInput, inputCmd = m.InstallInput.Update(msg)
	afterRaw := m.InstallInput.Value()
	after := strings.TrimSpace(afterRaw)

	if before == afterRaw {
		return m, inputCmd
	}

	if after == "" {
		m.Suggestions = nil
		m.SuggestionSelected = 0
		m.SuggestionScroll = 0
		m.ModalLoadingSuggestions = false
		m.ModalLastQuery = ""
		return m, inputCmd
	}

	m.ModalLastQuery = after
	m.ModalLoadingSuggestions = true
	searchCmd := m.searchSuggestionsCmd(after)
	if inputCmd == nil {
		return m, searchCmd
	}
	return m, tea.Batch(inputCmd, searchCmd)
}

func (m ViewModel) View() string {
	if m.Width == 0 || m.Height == 0 {
		return ""
	}

	logHeight := 3
	helpHeight := 1
	bodyHeight := m.Height - logHeight - helpHeight
	if bodyHeight < 8 {
		bodyHeight = 8
	}

	// Total rendered width is: left( +2 border ) + separator(1) + right( +2 border ).
	availableContentWidth := m.Width - 6
	if availableContentWidth < 20 {
		availableContentWidth = 20
	}
	leftWidth := availableContentWidth / 2
	rightWidth := availableContentWidth - leftWidth
	if leftWidth < 10 {
		leftWidth = 10
	}
	if rightWidth < 10 {
		rightWidth = 10
	}

	leftStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(leftWidth).Height(bodyHeight - 2)
	rightStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(rightWidth).Height(bodyHeight - 2)

	leftPane := leftStyle.Render(m.renderInstalledPackages(leftWidth-4, bodyHeight-4))
	rightPane := rightStyle.Render(m.renderPackageDetails(rightWidth - 4))
	mainPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)

	baseMain := lipgloss.JoinVertical(lipgloss.Left, mainPanes, m.renderLogLine())
	helpLine := m.renderBottomHelp()
	if !m.ModalOpen {
		return lipgloss.JoinVertical(lipgloss.Left, baseMain, helpLine)
	}

	modal := m.renderInstallModal()
	plainModal := stripANSI(modal)
	x := (m.Width - lipgloss.Width(plainModal)) / 2
	if x < 0 {
		x = 0
	}
	y := (m.Height - lipgloss.Height(plainModal)) / 2
	if y < 0 {
		y = 0
	}

	overlaid := overlayAt(stripANSI(baseMain), plainModal, x, y)
	return lipgloss.JoinVertical(lipgloss.Left, overlaid, helpLine)
}

func stripANSI(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}

func overlayAt(base string, overlay string, x int, y int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	for i, line := range overlayLines {
		target := y + i
		if target < 0 || target >= len(baseLines) {
			continue
		}

		baseRunes := []rune(baseLines[target])
		overlayRunes := []rune(line)
		if len(overlayRunes) == 0 {
			continue
		}

		needed := x + len(overlayRunes)
		if len(baseRunes) < needed {
			baseRunes = append(baseRunes, []rune(strings.Repeat(" ", needed-len(baseRunes)))...)
		}

		copy(baseRunes[x:needed], overlayRunes)
		baseLines[target] = string(baseRunes)
	}

	return strings.Join(baseLines, "\n")
}

func (m *ViewModel) openModal() {
	m.ModalOpen = true
	m.InstallInput.SetValue("")
	m.InstallInput.Focus()
	m.Suggestions = nil
	m.SuggestionSelected = 0
	m.SuggestionScroll = 0
	m.ModalLastQuery = ""
	m.ModalLoadingSuggestions = false
}

func (m *ViewModel) closeModal() {
	m.ModalOpen = false
	m.InstallInput.Blur()
	m.InstallInput.SetValue("")
	m.Suggestions = nil
	m.SuggestionSelected = 0
	m.SuggestionScroll = 0
	m.ModalLastQuery = ""
	m.ModalLoadingSuggestions = false
}

func (m *ViewModel) setLog(kind logKind, text string) {
	m.LogKind = kind
	m.LogText = strings.TrimSpace(text)
	if m.LogText == "" {
		m.LogText = "Ready"
	}
}

func (m ViewModel) loadInstalledCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		deps, err := m.PackageManager.List(ctx)
		return listLoadedMsg{Packages: deps, Err: err}
	}
}

func (m ViewModel) uninstallCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.PackageManager.Remove(ctx, name)
		return uninstallDoneMsg{Name: name, Err: err}
	}
}

func (m ViewModel) searchSuggestionsCmd(query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		results, err := m.PackageManager.Search(ctx, query)
		return searchSuggestionsDoneMsg{Query: query, Results: results, Err: err}
	}
}

func (m ViewModel) installCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := m.PackageManager.Install(ctx, name)
		return installDoneMsg{Name: name, Err: err}
	}
}

func (m *ViewModel) ensureMainSelectionVisible(visibleRows int) {
	if visibleRows < 1 {
		visibleRows = 1
	}
	if m.Selected < m.Scroll {
		m.Scroll = m.Selected
	}
	if m.Selected >= m.Scroll+visibleRows {
		m.Scroll = m.Selected - visibleRows + 1
	}
	if m.Scroll < 0 {
		m.Scroll = 0
	}
	maxScroll := len(m.Packages) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.Scroll > maxScroll {
		m.Scroll = maxScroll
	}
}

func (m *ViewModel) ensureSuggestionSelectionVisible(visibleRows int) {
	if visibleRows < 1 {
		visibleRows = 1
	}
	if m.SuggestionSelected < m.SuggestionScroll {
		m.SuggestionScroll = m.SuggestionSelected
	}
	if m.SuggestionSelected >= m.SuggestionScroll+visibleRows {
		m.SuggestionScroll = m.SuggestionSelected - visibleRows + 1
	}
	if m.SuggestionScroll < 0 {
		m.SuggestionScroll = 0
	}
	maxScroll := len(m.Suggestions) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.SuggestionScroll > maxScroll {
		m.SuggestionScroll = maxScroll
	}
}

func (m ViewModel) visibleMainRows() int {
	rows := m.Height - 8
	if rows < 4 {
		rows = 4
	}
	return rows
}

func (m ViewModel) visibleSuggestionRows() int {
	rows := int(float64(m.Height) * 0.7)
	rows -= 7
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m ViewModel) renderInstalledPackages(width int, rows int) string {
	if width < 10 {
		width = 10
	}
	if rows < 3 {
		rows = 3
	}

	header := lipgloss.NewStyle().Bold(true).Render("Installed Packages")

	if m.LoadingList {
		return strings.Join([]string{header, "", "Loading installed packages..."}, "\n")
	}
	if len(m.Packages) == 0 {
		return strings.Join([]string{header, "", "No installed packages found."}, "\n")
	}

	start := m.Scroll
	if start < 0 {
		start = 0
	}
	if start >= len(m.Packages) {
		start = len(m.Packages) - 1
	}
	if start < 0 {
		start = 0
	}

	end := start + rows
	if end > len(m.Packages) {
		end = len(m.Packages)
	}

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("57"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	lines := []string{header, ""}
	for i := start; i < end; i++ {
		dep := m.Packages[i]
		line := dep.Name
		if dep.Version != "" {
			line += " " + dep.Version
		}
		line = TruncateText(line, width)
		if i == m.Selected {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}

	lines = append(lines, "", mutedStyle.Render(fmt.Sprintf("%d/%d", m.Selected+1, len(m.Packages))))
	return strings.Join(lines, "\n")
}

func (m ViewModel) renderPackageDetails(width int) string {
	if width < 10 {
		width = 10
	}

	header := lipgloss.NewStyle().Bold(true).Render("Package Details")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true)

	if m.LoadingList {
		return strings.Join([]string{header, "", muted.Render("Loading...")}, "\n")
	}
	if len(m.Packages) == 0 || m.Selected < 0 || m.Selected >= len(m.Packages) {
		return strings.Join([]string{header, "", muted.Render("No package selected.")}, "\n")
	}

	dep := m.Packages[m.Selected]
	version := strings.TrimSpace(dep.Version)
	if version == "" {
		version = "unknown"
	}

	lines := []string{
		header,
		"",
		accent.Render(dep.Name),
		muted.Render("Version"),
		WrapText(version, width),
	}

	return strings.Join(lines, "\n")
}

func (m ViewModel) renderInstallModal() string {
	modalWidth := int(float64(m.Width) * 0.7)
	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalWidth > m.Width-2 {
		modalWidth = m.Width - 2
	}

	modalHeight := int(float64(m.Height) * 0.7)
	if modalHeight < 10 {
		modalHeight = 10
	}
	if modalHeight > m.Height-2 {
		modalHeight = m.Height - 2
	}
	modalStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(modalWidth).Height(modalHeight)
	header := lipgloss.NewStyle().Bold(true).Render("Install Package")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	subtitle := muted.Render("Type to search")
	innerHeight := modalHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	rows := m.visibleSuggestionRows()
	start := m.SuggestionScroll
	if start < 0 {
		start = 0
	}
	if start >= len(m.Suggestions) {
		start = len(m.Suggestions) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + rows
	if end > len(m.Suggestions) {
		end = len(m.Suggestions)
	}

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("57"))

	suggestionLines := make([]string, 0, rows)
	if m.ModalLoadingSuggestions {
		suggestionLines = append(suggestionLines, muted.Render("Searching suggestions..."))
	} else if len(m.Suggestions) == 0 {
		suggestionLines = append(suggestionLines, muted.Render("No suggestions"))
	} else {
		for i := start; i < end; i++ {
			line := TruncateText(m.Suggestions[i].Name, modalWidth-8)
			if i == m.SuggestionSelected {
				line = selectedStyle.Render(line)
			}
			suggestionLines = append(suggestionLines, line)
		}
	}

	body := []string{header, m.InstallInput.View(), subtitle}
	body = append(body, suggestionLines...)
	if len(body) > innerHeight {
		body = body[:innerHeight]
	} else {
		for len(body) < innerHeight {
			body = append(body, "")
		}
	}

	return modalStyle.Render(strings.Join(body, "\n"))
}

func (m ViewModel) renderLogLine() string {
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(m.Width - 2).Height(1)
	text := m.LogText
	if text == "" {
		text = "Ready"
	}

	prefix := "[INFO] "
	color := lipgloss.Color("245")
	switch m.LogKind {
	case logSuccess:
		prefix = "[OK] "
		color = lipgloss.Color("42")
	case logError:
		prefix = "[ERR] "
		color = lipgloss.Color("196")
	case logLoading:
		prefix = "[LOAD] "
		color = lipgloss.Color("220")
	}

	line := lipgloss.NewStyle().Foreground(color).Render(prefix + text)
	line = TruncateText(line, m.Width-6)
	return style.Render(line)
}

func (m ViewModel) renderBottomHelp() string {
	help := "ESC back | Up/Down select | D uninstall | I open install"
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	if m.ModalOpen {
		help = "ESC close modal | Up/Down select suggestion | I install selected/typed"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	}

	return style.Render(TruncateText(help, m.Width))
}
