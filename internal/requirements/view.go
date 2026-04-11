package requirements

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	packagemanager "pipnest/internal/requirements/package_manager"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type logKind string

const (
	logInfo    logKind = "info"
	logSuccess logKind = "success"
	logError   logKind = "error"
	logLoading logKind = "loading"
)

var (
	reqMutedColor   = lipgloss.Color("8")
	reqGlobalColor  = lipgloss.Color("6")
	reqVenvColor    = lipgloss.Color("3")
	reqTitleColor   = lipgloss.Color("5")
	reqValueColor   = lipgloss.Color("4")
	reqKeyColor     = lipgloss.Color("2")
	reqVersionColor = lipgloss.Color("1")
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
	ModalErrorText          string
	ModalLoading            bool

	ActionModalOpen    bool
	ActionModalLoading bool
	ActionModalTitle   string
	ActionModalText    string
	ActionModalKind    logKind
	HelpModalOpen      bool

	LogText string
	LogKind logKind

	SelectedMeta        *Result
	SelectedMetaLoading bool
	SelectedMetaErr     string
	DetailsScroll       int
	FocusedPane         int // 0 = list, 1 = details
	metaCache           map[string]Result
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

type freezeDoneMsg struct {
	FilePath  string
	Err       error
	ShowModal bool
}

type installFromFileDoneMsg struct {
	FilePath string
	Err      error
}

type packageMetaLoadedMsg struct {
	Name string
	Meta Result
	Err  error
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var glowRenderCache = map[string]string{}
var glamourRendererCache = map[int]*glamour.TermRenderer{}

func NewViewModel() ViewModel {
	installInput := textinput.New()
	installInput.Placeholder = "Type package name..."

	pm := packagemanager.PackageManager(packagemanager.NewPipManager("pip"))
	logText := "Loading installed packages... (pip)"
	if _, err := exec.LookPath("uv"); err == nil {
		pm = packagemanager.NewUVManager("uv")
		logText = "Loading installed packages... (uv)"
	}

	return ViewModel{
		PackageManager: pm,
		InstallInput:   installInput,
		LogText:        logText,
		LogKind:        logLoading,
		metaCache:      map[string]Result{},
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
		if m.ActionModalOpen {
			return m.updateActionModal(msg)
		}
		if m.HelpModalOpen {
			return m.updateHelpModal(msg)
		}
		return m.updateMainWindow(msg)
	case tea.MouseMsg:
		if m.ModalOpen || m.ActionModalOpen || m.HelpModalOpen {
			return m, nil
		}
		return m.updateMainMouse(msg)
	case listLoadedMsg:
		m.LoadingList = false
		if msg.Err != nil {
			m.Packages = nil
			m.Selected = 0
			m.Scroll = 0
			m.SelectedMeta = nil
			m.SelectedMetaErr = ""
			m.SelectedMetaLoading = false
			m.DetailsScroll = 0
			m.setLog(logError, "Failed to load installed packages: "+msg.Err.Error())
			return m, nil
		}

		m.Packages = msg.Packages
		if len(m.Packages) == 0 {
			m.Selected = 0
			m.Scroll = 0
			m.SelectedMeta = nil
			m.SelectedMetaErr = ""
			m.SelectedMetaLoading = false
			m.DetailsScroll = 0
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
		if len(m.Packages) > 0 {
			m, cmd := m.beginSelectedPackageMetaLoad()
			return m, cmd
		}
		return m, nil
	case uninstallDoneMsg:
		m.BusyAction = false
		if msg.Err != nil {
			m.showActionModalResult(logError, "Uninstall failed", msg.Err.Error())
			m.setLog(logError, "Uninstall failed: "+msg.Err.Error())
			return m, nil
		}

		m.showActionModalResult(logSuccess, "Uninstall completed", "Removed: "+msg.Name)
		m.setLog(logSuccess, fmt.Sprintf("Uninstalled %s (auto-freeze running)", msg.Name))
		m.LoadingList = true
		freezePath, freezeErr := requirementsOutputPath()
		if freezeErr != nil {
			m.setLog(logError, "Auto-freeze failed: "+freezeErr.Error())
			return m, m.loadInstalledCmd()
		}
		return m, tea.Batch(m.loadInstalledCmd(), m.freezeCmd(freezePath, false))
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
		m.ModalLoading = false
		if msg.Err != nil {
			m.ModalErrorText = "Install failed: " + msg.Err.Error()
			m.setLog(logError, "Install failed: "+msg.Err.Error())
			return m, nil
		}

		m.closeModal()
		m.setLog(logSuccess, fmt.Sprintf("Installed %s (auto-freeze running)", msg.Name))
		m.LoadingList = true
		freezePath, freezeErr := requirementsOutputPath()
		if freezeErr != nil {
			m.setLog(logError, "Auto-freeze failed: "+freezeErr.Error())
			return m, m.loadInstalledCmd()
		}
		return m, tea.Batch(m.loadInstalledCmd(), m.freezeCmd(freezePath, false))
	case freezeDoneMsg:
		m.BusyAction = false
		if msg.Err != nil {
			if msg.ShowModal {
				m.showActionModalResult(logError, "Freeze failed", msg.Err.Error())
			}
			m.setLog(logError, "Freeze failed: "+msg.Err.Error())
			return m, nil
		}
		if msg.ShowModal {
			m.showActionModalResult(logSuccess, "Freeze completed", "Updated: "+msg.FilePath)
		}
		m.setLog(logSuccess, fmt.Sprintf("requirements.txt updated: %s", msg.FilePath))
		return m, nil
	case installFromFileDoneMsg:
		m.BusyAction = false
		if msg.Err != nil {
			m.showActionModalResult(logError, "Install failed", msg.Err.Error())
			m.setLog(logError, "Install from requirements failed: "+msg.Err.Error())
			return m, nil
		}
		m.showActionModalResult(logSuccess, "Install completed", "Installed from: "+msg.FilePath)
		m.setLog(logSuccess, fmt.Sprintf("Installed from %s", msg.FilePath))
		m.LoadingList = true
		return m, m.loadInstalledCmd()
	case packageMetaLoadedMsg:
		selectedName := m.selectedPackageName()
		if msg.Name == "" || selectedName == "" || !strings.EqualFold(strings.TrimSpace(selectedName), strings.TrimSpace(msg.Name)) {
			return m, nil
		}

		m.SelectedMetaLoading = false
		if msg.Err != nil {
			m.SelectedMeta = nil
			m.SelectedMetaErr = msg.Err.Error()
			return m, nil
		}

		m.metaCache[strings.ToLower(strings.TrimSpace(msg.Name))] = msg.Meta
		meta := msg.Meta
		m.SelectedMeta = &meta
		m.SelectedMetaErr = ""
		return m, nil
	}

	return m, nil
}

func (m ViewModel) updateMainWindow(msg tea.KeyMsg) (ViewModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyLeft:
		m.FocusedPane = 0
		return m, nil
	case tea.KeyRight:
		m.FocusedPane = 1
		return m, nil
	case tea.KeyUp, tea.KeyCtrlP:
		if m.FocusedPane == 1 {
			m.DetailsScroll -= 3
			if m.DetailsScroll < 0 {
				m.DetailsScroll = 0
			}
			return m, nil
		}
		if m.Selected > 0 {
			m.Selected--
			m.ensureMainSelectionVisible(m.visibleMainRows())
			m.DetailsScroll = 0
			m, cmd := m.beginSelectedPackageMetaLoad()
			return m, cmd
		}
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if m.FocusedPane == 1 {
			m.DetailsScroll += 3
			return m, nil
		}
		if m.Selected < len(m.Packages)-1 {
			m.Selected++
			m.ensureMainSelectionVisible(m.visibleMainRows())
			m.DetailsScroll = 0
			m, cmd := m.beginSelectedPackageMetaLoad()
			return m, cmd
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
		m.DetailsScroll = 0
		m, cmd := m.beginSelectedPackageMetaLoad()
		return m, cmd
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
		m.DetailsScroll = 0
		m, cmd := m.beginSelectedPackageMetaLoad()
		return m, cmd
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return m, nil
		}

		switch msg.Runes[0] {
		case 'j', 'J':
			if m.Selected < len(m.Packages)-1 {
				m.Selected++
				m.ensureMainSelectionVisible(m.visibleMainRows())
				m.DetailsScroll = 0
				m, cmd := m.beginSelectedPackageMetaLoad()
				return m, cmd
			}
			return m, nil
		case 'k', 'K':
			if m.Selected > 0 {
				m.Selected--
				m.ensureMainSelectionVisible(m.visibleMainRows())
				m.DetailsScroll = 0
				m, cmd := m.beginSelectedPackageMetaLoad()
				return m, cmd
			}
			return m, nil
		case 'u':
			m.DetailsScroll -= 4
			if m.DetailsScroll < 0 {
				m.DetailsScroll = 0
			}
			return m, nil
		case 'U':
			m.DetailsScroll -= 12
			if m.DetailsScroll < 0 {
				m.DetailsScroll = 0
			}
			return m, nil
		case 'i', 'I':
			m.openModal()
			m.setLog(logInfo, "Install mode opened")
			return m, nil
		case '?':
			m.openHelpModal()
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
			m.showActionModalLoading("Uninstall package", fmt.Sprintf("Uninstalling %s...", name))
			m.setLog(logLoading, fmt.Sprintf("Uninstalling %s...", name))
			return m, m.uninstallCmd(name)
		case 'l', 'L':
			if m.BusyAction {
				return m, nil
			}
			m.LoadingList = true
			m.setLog(logLoading, "Refreshing installed packages...")
			return m, m.loadInstalledCmd()
		case 'f', 'F':
			if m.BusyAction || m.LoadingList {
				return m, nil
			}
			freezePath, err := requirementsOutputPath()
			if err != nil {
				m.setLog(logError, "Freeze failed: "+err.Error())
				return m, nil
			}
			m.BusyAction = true
			m.showActionModalLoading("Freeze requirements", "Running pip freeze into requirements.txt...")
			m.setLog(logLoading, "Running freeze to requirements.txt...")
			return m, m.freezeCmd(freezePath, true)
		case 'r', 'R':
			if m.BusyAction || m.LoadingList {
				return m, nil
			}
			reqFile, ok := findNearestRequirementsFile()
			if !ok {
				m.showActionModalResult(logInfo, "requirements.txt not found", "No requirements.txt found in this project tree")
				m.setLog(logInfo, "No requirements.txt found in project")
				return m, nil
			}
			m.BusyAction = true
			m.showActionModalLoading("Install requirements", "Installing packages from requirements.txt...")
			m.setLog(logLoading, "Installing from requirements.txt...")
			return m, m.installFromFileCmd(reqFile)
		}
	}

	return m, nil
}

func (m ViewModel) updateMainMouse(msg tea.MouseMsg) (ViewModel, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		m.DetailsScroll--
		if m.DetailsScroll < 0 {
			m.DetailsScroll = 0
		}
	case tea.MouseWheelDown:
		m.DetailsScroll++
	}

	return m, nil
}

func (m ViewModel) updateInstallModal(msg tea.KeyMsg) (ViewModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.closeModal()
		m.setLog(logInfo, "Install mode closed")
		return m, nil
	case tea.KeyEnter:
		if m.BusyAction {
			return m, nil
		}

		m.ModalErrorText = ""
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
		m.ModalLoading = true
		m.ModalErrorText = ""
		m.setLog(logLoading, fmt.Sprintf("Installing %s...", pkgName))
		return m, m.installCmd(pkgName)
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
		m.ModalErrorText = ""
		return m, inputCmd
	}

	m.ModalLastQuery = after
	m.ModalLoadingSuggestions = true
	m.ModalErrorText = ""
	searchCmd := m.searchSuggestionsCmd(after)
	if inputCmd == nil {
		return m, searchCmd
	}
	return m, tea.Batch(inputCmd, searchCmd)
}

func (m ViewModel) updateActionModal(msg tea.KeyMsg) (ViewModel, tea.Cmd) {
	if m.ActionModalLoading {
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.closeActionModal()
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q', 'Q':
				m.closeActionModal()
				return m, nil
			}
		}
	}

	return m, nil
}

func (m ViewModel) updateHelpModal(msg tea.KeyMsg) (ViewModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.closeHelpModal()
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case '?', 'q', 'Q':
				m.closeHelpModal()
				return m, nil
			}
		}
	}

	return m, nil
}

func (m ViewModel) View() string {
	if m.Width == 0 || m.Height == 0 {
		return ""
	}

	helpHeight := 1
	bodyHeight := m.Height - helpHeight
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

	leftBorderColor := reqMutedColor
	rightBorderColor := reqMutedColor
	if m.FocusedPane == 0 {
		leftBorderColor = reqTitleColor
	} else {
		rightBorderColor = reqTitleColor
	}

	leftStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(leftBorderColor).
		Width(leftWidth).
		Height(bodyHeight - 2)
	rightStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rightBorderColor).
		Width(rightWidth).
		Height(bodyHeight - 2)

	listRows := bodyHeight - 6
	if listRows < 3 {
		listRows = 3
	}

	leftPane := leftStyle.Render(m.renderInstalledPackages(leftWidth-4, listRows))
	if len(m.Packages) > listRows {
		leftPane = overlayScrollbarOnBorder(leftPane, len(m.Packages), m.Scroll, listRows)
	}

	rightRows := bodyHeight - 4
	if rightRows < 4 {
		rightRows = 4
	}
	detailContent, detailTotal, detailScroll := m.renderPackageDetails(rightWidth-4, rightRows)
	rightPane := rightStyle.Render(detailContent)
	if detailTotal > rightRows {
		rightPane = overlayScrollbarOnBorder(rightPane, detailTotal, detailScroll, rightRows)
	}

	mainPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)

	baseMain := mainPanes
	helpLine := m.renderBottomHelp()
	if !m.ModalOpen && !m.ActionModalOpen && !m.HelpModalOpen {
		return lipgloss.JoinVertical(lipgloss.Left, baseMain, helpLine)
	}

	modal := ""
	if m.ModalOpen {
		modal = m.renderInstallModal()
	} else if m.HelpModalOpen {
		modal = m.renderHelpModal()
	} else {
		modal = m.renderActionModal()
	}
	plainModal := stripANSI(modal)
	x := (m.Width - lipgloss.Width(plainModal)) / 2
	if x < 0 {
		x = 0
	}
	y := (m.Height - lipgloss.Height(plainModal)) / 2
	if y < 0 {
		y = 0
	}

	overlaid := overlayAt(stripANSI(baseMain), modal, x, y)
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
		overlayVisibleRunes := []rune(stripANSI(line))
		overlayWidth := len(overlayVisibleRunes)
		if overlayWidth == 0 {
			continue
		}

		needed := x + overlayWidth
		if len(baseRunes) < needed {
			baseRunes = append(baseRunes, []rune(strings.Repeat(" ", needed-len(baseRunes)))...)
		}

		prefix := string(baseRunes[:x])
		suffix := ""
		if needed < len(baseRunes) {
			suffix = string(baseRunes[needed:])
		}
		baseLines[target] = prefix + line + suffix
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
	m.ModalErrorText = ""
	m.ModalLoading = false
}

func (m *ViewModel) showActionModalLoading(title string, text string) {
	m.ActionModalOpen = true
	m.ActionModalLoading = true
	m.ActionModalTitle = strings.TrimSpace(title)
	m.ActionModalText = strings.TrimSpace(text)
	m.ActionModalKind = logLoading
}

func (m *ViewModel) showActionModalResult(kind logKind, title string, text string) {
	m.ActionModalOpen = true
	m.ActionModalLoading = false
	m.ActionModalTitle = strings.TrimSpace(title)
	m.ActionModalText = strings.TrimSpace(text)
	m.ActionModalKind = kind
}

func (m *ViewModel) closeActionModal() {
	m.ActionModalOpen = false
	m.ActionModalLoading = false
	m.ActionModalTitle = ""
	m.ActionModalText = ""
	m.ActionModalKind = logInfo
}

func (m *ViewModel) openHelpModal() {
	m.HelpModalOpen = true
}

func (m *ViewModel) closeHelpModal() {
	m.HelpModalOpen = false
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

func (m ViewModel) freezeCmd(filePath string, showModal bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := m.PackageManager.Freeze(ctx, filePath)
		return freezeDoneMsg{FilePath: filePath, Err: err, ShowModal: showModal}
	}
}

func (m ViewModel) installFromFileCmd(filePath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		err := m.PackageManager.InstallFromFile(ctx, filePath)
		return installFromFileDoneMsg{FilePath: filePath, Err: err}
	}
}

func (m ViewModel) beginSelectedPackageMetaLoad() (ViewModel, tea.Cmd) {
	name := m.selectedPackageName()
	if name == "" {
		m.SelectedMeta = nil
		m.SelectedMetaErr = ""
		m.SelectedMetaLoading = false
		return m, nil
	}
	cacheKey := strings.ToLower(strings.TrimSpace(name))
	if cached, ok := m.metaCache[cacheKey]; ok {
		meta := cached
		m.SelectedMeta = &meta
		m.SelectedMetaErr = ""
		m.SelectedMetaLoading = false
		return m, nil
	}

	m.SelectedMeta = nil
	m.SelectedMetaErr = ""
	m.SelectedMetaLoading = true

	cmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		type metaRes struct {
			meta Result
			err  error
		}
		ch := make(chan metaRes, 1)
		go func() {
			meta, err := fetchPackageMetadata(name)
			ch <- metaRes{meta: meta, err: err}
		}()

		select {
		case <-ctx.Done():
			return packageMetaLoadedMsg{Name: name, Err: ctx.Err()}
		case res := <-ch:
			return packageMetaLoadedMsg{Name: name, Meta: res.meta, Err: res.err}
		}
	}

	return m, cmd
}

func (m ViewModel) selectedPackageName() string {
	if len(m.Packages) == 0 || m.Selected < 0 || m.Selected >= len(m.Packages) {
		return ""
	}
	return strings.TrimSpace(m.Packages[m.Selected].Name)
}

func requirementsOutputPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, "requirements.txt"), nil
}

func findNearestRequirementsFile() (string, bool) {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return "", false
	}
	current := wd
	for {
		candidate := filepath.Join(current, "requirements.txt")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", false
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

	header := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Installed Packages")
	loadingStyle := lipgloss.NewStyle().Foreground(reqVenvColor)
	emptyStyle := lipgloss.NewStyle().Foreground(reqMutedColor)

	if m.LoadingList {
		return strings.Join([]string{header, "", loadingStyle.Render("Loading installed packages...")}, "\n")
	}
	if len(m.Packages) == 0 {
		return strings.Join([]string{header, "", emptyStyle.Render("No installed packages found.")}, "\n")
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

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(reqVenvColor).Reverse(true)
	mutedStyle := lipgloss.NewStyle().Foreground(reqMutedColor)

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

func (m ViewModel) renderPackageDetails(width int, rows int) (string, int, int) {
	if width < 10 {
		width = 10
	}
	if rows < 4 {
		rows = 4
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Package Details")
	muted := lipgloss.NewStyle().Foreground(reqMutedColor)
	accent := lipgloss.NewStyle().Foreground(reqValueColor).Bold(true)

	if m.LoadingList {
		return strings.Join([]string{header, "", muted.Render("Loading...")}, "\n"), 0, 0
	}
	if len(m.Packages) == 0 || m.Selected < 0 || m.Selected >= len(m.Packages) {
		return strings.Join([]string{header, "", muted.Render("No package selected.")}, "\n"), 0, 0
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
		lipgloss.NewStyle().Foreground(reqKeyColor).Render("Version"),
		WrapText(version, width),
	}

	if m.SelectedMetaLoading {
		lines = append(lines, "", muted.Render("Loading package metadata from PyPI..."))
		return m.renderScrollableDetails(lines, width, rows)
	}

	if m.SelectedMetaErr != "" {
		lines = append(lines,
			"",
			lipgloss.NewStyle().Foreground(reqVersionColor).Render("Metadata unavailable"),
			WrapText(m.SelectedMetaErr, width),
		)
		return m.renderScrollableDetails(lines, width, rows)
	}

	if m.SelectedMeta != nil {
		summary := strings.TrimSpace(m.SelectedMeta.Description)
		if summary != "" {
			lines = append(lines, "", lipgloss.NewStyle().Foreground(reqKeyColor).Render("Summary"), WrapText(summary, width))
		}
		if strings.TrimSpace(m.SelectedMeta.Readme) != "" {
			lines = append(lines, "", lipgloss.NewStyle().Foreground(reqKeyColor).Render("README"), m.renderMarkdownWithGlamour(m.SelectedMeta.Readme, width))
		}
	}

	return m.renderScrollableDetails(lines, width, rows)
}

func (m ViewModel) renderScrollableDetails(lines []string, width int, rows int) (string, int, int) {
	flat := make([]string, 0, len(lines))
	for _, line := range lines {
		flat = append(flat, strings.Split(line, "\n")...)
	}

	maxScroll := 0
	if len(flat) > rows {
		maxScroll = len(flat) - rows
	}
	if m.DetailsScroll < 0 {
		m.DetailsScroll = 0
	}
	if m.DetailsScroll > maxScroll {
		m.DetailsScroll = maxScroll
	}

	start := m.DetailsScroll
	end := start + rows
	if end > len(flat) {
		end = len(flat)
	}

	out := append([]string{}, flat[start:end]...)
	for len(out) < rows {
		out = append(out, "")
	}

	return strings.Join(out, "\n"), len(flat), start
}

func (m ViewModel) renderMarkdownWithGlamour(markdown string, width int) string {
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

	renderer, err := getGlamourRenderer(width)
	if err != nil {
		rendered := wrapMarkdownFallback(md, width)
		glowRenderCache[cacheKey] = rendered
		return rendered
	}
	rendered, renderErr := renderer.Render(md)
	if renderErr != nil {
		rendered := wrapMarkdownFallback(md, width)
		glowRenderCache[cacheKey] = rendered
		return rendered
	}

	rendered = strings.TrimRight(rendered, "\n")
	if strings.TrimSpace(rendered) == "" {
		rendered = wrapMarkdownFallback(md, width)
	}
	glowRenderCache[cacheKey] = rendered
	return rendered
}

func getGlamourRenderer(width int) (*glamour.TermRenderer, error) {
	if width < 20 {
		width = 20
	}
	if cached, ok := glamourRendererCache[width]; ok {
		return cached, nil
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}
	glamourRendererCache[width] = renderer
	return renderer, nil
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
		out = append(out, strings.Split(WrapText(trimmed, width), "\n")...)
	}
	return strings.Join(out, "\n")
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
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(reqTitleColor).
		Width(modalWidth).
		Height(modalHeight)
	header := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Install Package")
	muted := lipgloss.NewStyle().Foreground(reqMutedColor)
	subtitle := lipgloss.NewStyle().Foreground(reqGlobalColor).Render("Type to search")
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

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(reqVenvColor).Reverse(true)
	inputStyle := lipgloss.NewStyle().Foreground(reqValueColor).Bold(true)

	suggestionLines := make([]string, 0, rows)
	if m.ModalLoadingSuggestions {
		suggestionLines = append(suggestionLines, lipgloss.NewStyle().Foreground(reqVenvColor).Render("Searching suggestions..."))
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

	body := []string{header, inputStyle.Render(m.InstallInput.View()), subtitle}
	body = append(body, suggestionLines...)

	errorBlock := ""
	errorLinesCount := 0
	if m.ModalErrorText != "" {
		wrapped := WrapText(m.ModalErrorText, modalWidth-8)
		errorBlock = lipgloss.NewStyle().Foreground(reqVersionColor).Render(wrapped)
		errorLinesCount = len(strings.Split(wrapped, "\n"))
	} else if m.ModalLoading {
		errorBlock = lipgloss.NewStyle().Foreground(reqVenvColor).Render("Installing...")
		errorLinesCount = 1
	}

	contentHeight := innerHeight
	if errorLinesCount > 0 {
		contentHeight = innerHeight - errorLinesCount
		if contentHeight < 0 {
			contentHeight = 0
		}
	}

	if len(body) > contentHeight {
		body = body[:contentHeight]
	} else {
		for len(body) < contentHeight {
			body = append(body, "")
		}
	}

	if errorBlock != "" {
		body = append(body, strings.Split(errorBlock, "\n")...)
	}

	return modalStyle.Render(strings.Join(body, "\n"))
}

func (m ViewModel) renderActionModal() string {
	modalWidth := int(float64(m.Width) * 0.62)
	if modalWidth < 36 {
		modalWidth = 36
	}
	if modalWidth > m.Width-2 {
		modalWidth = m.Width - 2
	}

	modalHeight := 10
	if modalHeight > m.Height-2 {
		modalHeight = m.Height - 2
	}
	if modalHeight < 7 {
		modalHeight = 7
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(reqTitleColor).
		Width(modalWidth).
		Height(modalHeight)

	titleColor := reqGlobalColor
	textColor := reqMutedColor
	if m.ActionModalLoading {
		titleColor = reqVenvColor
		textColor = reqGlobalColor
	} else {
		switch m.ActionModalKind {
		case logSuccess:
			titleColor = reqKeyColor
			textColor = reqGlobalColor
		case logError:
			titleColor = reqVersionColor
			textColor = reqMutedColor
		case logInfo:
			titleColor = reqGlobalColor
			textColor = reqMutedColor
		}
	}

	title := strings.TrimSpace(m.ActionModalTitle)
	if title == "" {
		title = "Action"
	}
	message := strings.TrimSpace(m.ActionModalText)
	if message == "" {
		message = "Working..."
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(TruncateText(title, modalWidth-6))
	body := lipgloss.NewStyle().Foreground(textColor).Render(WrapText(message, modalWidth-6))
	footerText := "Esc/Enter close"
	if m.ActionModalLoading {
		footerText = "Working..."
	}
	footer := lipgloss.NewStyle().Foreground(reqMutedColor).Render(footerText)

	lines := []string{header, "", body, "", footer}
	innerHeight := modalHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	} else {
		for len(lines) < innerHeight {
			lines = append(lines, "")
		}
	}

	return modalStyle.Render(strings.Join(lines, "\n"))
}

func (m ViewModel) renderHelpModal() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Keybinds")
	muted := lipgloss.NewStyle().Foreground(reqMutedColor)
	key := lipgloss.NewStyle().Bold(true).Foreground(reqKeyColor)
	detail := lipgloss.NewStyle().Foreground(reqValueColor)

	rows := []string{
		title,
		muted.Render("Core"),
		key.Render("i") + detail.Render("  install package"),
		key.Render("d") + detail.Render("  uninstall selected package"),
		key.Render("j/k or ↑/↓") + detail.Render("  move in package list"),
		key.Render("PgUp/PgDown") + detail.Render("  jump in package list"),
		key.Render("Ctrl+U/Ctrl+D") + detail.Render("  scroll details/readme"),
		key.Render("Esc") + detail.Render("  return to menu"),
		key.Render("q") + detail.Render("  quit app"),
		"",
		muted.Render("Secondary"),
		key.Render("f") + detail.Render("  freeze into requirements.txt"),
		key.Render("r") + detail.Render("  install from nearest requirements.txt"),
		key.Render("l") + detail.Render("  refresh installed packages"),
		"",
		muted.Render("Close help: Esc, Enter, ?, or q"),
	}

	modalWidth := min(max(56, m.Width-12), 90)
	if modalWidth < 56 {
		modalWidth = 56
	}

	modalHeight := min(max(10, len(rows)+2), max(10, m.Height-2))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(reqTitleColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight).
		Render(strings.Join(rows, "\n"))
}

func (m ViewModel) renderLogLine() string {
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width(m.Width - 2).Height(1)
	text := m.LogText
	if text == "" {
		text = "Ready"
	}

	prefix := "[INFO] "
	color := reqMutedColor
	switch m.LogKind {
	case logSuccess:
		prefix = "[OK] "
		color = reqKeyColor
	case logError:
		prefix = "[ERR] "
		color = reqVersionColor
	case logLoading:
		prefix = "[LOAD] "
		color = reqVenvColor
	}

	line := lipgloss.NewStyle().Foreground(color).Render(prefix + text)
	line = TruncateText(line, m.Width-6)
	return style.Render(line)
}

func (m ViewModel) renderBottomHelp() string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(reqKeyColor)
	sepStyle := lipgloss.NewStyle().Foreground(reqMutedColor)

	if m.ModalOpen {
		legend := lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render("Enter"), sepStyle.Render(": install"),
			sepStyle.Render("  |  "),
			keyStyle.Render("Esc"), sepStyle.Render(": close"),
			sepStyle.Render("  |  "),
			keyStyle.Render("q"), sepStyle.Render(": quit"),
		)
		return TruncateText(legend, m.Width)
	}

	if m.ActionModalOpen {
		state := "result"
		if m.ActionModalLoading {
			state = "running"
		}
		legend := lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render("Action"), sepStyle.Render(": "+state),
			sepStyle.Render("  |  "),
			keyStyle.Render("Esc"), sepStyle.Render(": close"),
			sepStyle.Render("  |  "),
			keyStyle.Render("Enter"), sepStyle.Render(": close"),
		)
		return TruncateText(legend, m.Width)
	}

	if m.HelpModalOpen {
		legend := lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render("Esc"), sepStyle.Render(": close"),
			sepStyle.Render("  |  "),
			keyStyle.Render("Enter"), sepStyle.Render(": close"),
			sepStyle.Render("  |  "),
			keyStyle.Render("?"), sepStyle.Render(": close"),
		)
		return TruncateText(legend, m.Width)
	}

	leftLegend := lipgloss.JoinHorizontal(lipgloss.Top,
		keyStyle.Render("i"), sepStyle.Render(": install"),
		sepStyle.Render("  |  "),
		keyStyle.Render("d"), sepStyle.Render(": uninstall"),
		sepStyle.Render("  |  "),
		keyStyle.Render("?"), sepStyle.Render(": more"),
		sepStyle.Render("  |  "),
		keyStyle.Render("Esc"), sepStyle.Render(": menu"),
		sepStyle.Render("  |  "),
		keyStyle.Render("q"), sepStyle.Render(": quit"),
	)
	rightLegend := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Foreground(reqGlobalColor).Render("global"),
		lipgloss.NewStyle().Render(" / "),
		lipgloss.NewStyle().Foreground(reqVenvColor).Render("venv"),
	)
	spacer := lipgloss.NewStyle().Width(max(0, m.Width-lipgloss.Width(leftLegend)-lipgloss.Width(rightLegend))).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, leftLegend, spacer, rightLegend)
}
