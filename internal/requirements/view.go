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

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type logKind string

type managerOption struct {
	Key       string
	Label     string
	Available bool
}

const (
	logInfo    logKind = "info"
	logSuccess logKind = "success"
	logError   logKind = "error"
	logLoading logKind = "loading"
	minWidth   int     = 72
	minHeight  int     = 14
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

	ModalOpen                  bool
	InstallInput               textinput.Model
	Suggestions                []packagemanager.Dependency
	SuggestionSelected         int
	SuggestionScroll           int
	ModalLoadingSuggestions    bool
	ModalLastQuery             string
	ModalSearchSeq             int
	ModalErrorText             string
	ModalLoading               bool
	ModalFocusedPane           int
	ModalSuggestionMeta        *Result
	ModalSuggestionMetaLoading bool
	ModalSuggestionMetaName    string
	ModalSuggestionMetaScroll  int
	InstallMeta                *Result
	InstallMetaLoading         bool
	InstallMetaErr             string
	ManagerModalOpen           bool
	ManagerOptions             []managerOption
	ManagerSelected            int
	ManagerScroll              int
	VersionModalOpen           bool
	VersionsList               []string
	VersionSelected            int
	VersionScroll              int
	VersionLoading             bool
	VersionPackageName         string
	VersionErrorText           string
	VersionFromMain            bool
	Loader                  spinner.Model

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

// IsAsyncMsg returns true if msg is one of the async messages produced by
// requirements commands. Used by the top-level model to forward these messages
// to the requirements sub-model even when another screen is active.
func IsAsyncMsg(msg interface{}) bool {
	switch msg.(type) {
	case listLoadedMsg, uninstallDoneMsg, searchSuggestionsDoneMsg,
		installDoneMsg, versionsDoneMsg, freezeDoneMsg,
		installFromFileDoneMsg, packageMetaLoadedMsg, modalSuggestionMetaLoadedMsg,
		modalSearchDebounceMsg:
		return true
	}
	return false
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

type modalSearchDebounceMsg struct {
	Query string
	Seq   int
}

type installDoneMsg struct {
	Name string
	Err  error
}

type versionsDoneMsg struct {
	Name     string
	Versions []string
	Err      error
}

type modalSuggestionMetaLoadedMsg struct {
	Name string
	Meta Result
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

type installPackageMetaLoadedMsg struct {
	Name string
	Meta Result
	Err  error
}

type openManagerModalMsg struct{}

func OpenManagerModalCmd() tea.Cmd {
	return func() tea.Msg {
		return openManagerModalMsg{}
	}
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var markdownImagePattern = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
var glowRenderCache = map[string]string{}
var glamourRendererCache = map[int]*glamour.TermRenderer{}

const readmePreviewMaxChars = 12000

func NewViewModel() ViewModel {
	installInput := textinput.New()
	installInput.Placeholder = "Type package name..."
	loader := spinner.New()
	loader.Spinner = spinner.Dot
	loader.Style = lipgloss.NewStyle().Foreground(reqVenvColor)

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
	case spinner.TickMsg:
		if !m.InstallMetaLoading && !m.SelectedMetaLoading && !m.ModalLoadingSuggestions {
			return m, nil
		}

		var cmd tea.Cmd
		m.Loader, cmd = m.Loader.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.ensureMainSelectionVisible(m.visibleMainRows())
		m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
		return m, nil
	case tea.KeyMsg:
		if m.ModalOpen && m.ManagerModalOpen {
			return m.updateManagerModal(msg)
		}
		if m.ModalOpen && m.VersionModalOpen {
			return m.updateVersionModal(msg)
		}
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
	case openManagerModalMsg:
		m.openManagerModal()
		m.setLog(logInfo, "Select package manager")
		return m, nil
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
		if len(msg.Results) == 0 {
			m.InstallMeta = nil
			m.InstallMetaLoading = false
			m.InstallMetaErr = ""
			return m, nil
		}

		m, cmd := m.beginInstallPackageMetaLoad(msg.Results[0].Name)
		return m, cmd
	case installDoneMsg:
		m.BusyAction = false
		m.ModalLoading = false
		m.ActionModalOpen = false
		if msg.Err != nil {
			if m.VersionModalOpen {
				m.VersionErrorText = "Install failed: " + msg.Err.Error()
			} else {
				m.ModalErrorText = "Install failed: " + msg.Err.Error()
			}
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
	case versionsDoneMsg:
		if !strings.EqualFold(strings.TrimSpace(msg.Name), strings.TrimSpace(m.VersionPackageName)) {
			return m, nil
		}

		m.VersionLoading = false
		if msg.Err != nil {
			m.VersionsList = nil
			m.VersionSelected = 0
			m.VersionScroll = 0
			m.VersionErrorText = "Failed to load versions: " + msg.Err.Error()
			m.setLog(logError, m.VersionErrorText)
			return m, nil
		}

		m.VersionsList = msg.Versions
		m.VersionSelected = 0
		m.VersionScroll = 0
		m.ensureVersionSelectionVisible(m.visibleVersionRows())
		m.VersionErrorText = ""
		m.setLog(logInfo, fmt.Sprintf("Loaded %d versions for %s", len(msg.Versions), msg.Name))
		return m, nil
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
	case installPackageMetaLoadedMsg:
		selectedName := m.installSelectedPackageName()
		if msg.Name == "" || selectedName == "" || !strings.EqualFold(strings.TrimSpace(selectedName), strings.TrimSpace(msg.Name)) {
			return m, nil
		}

		m.InstallMetaLoading = false
		if msg.Err != nil {
			m.InstallMeta = nil
			m.InstallMetaErr = msg.Err.Error()
			return m, nil
		}

		m.metaCache[strings.ToLower(strings.TrimSpace(msg.Name))] = msg.Meta
		meta := msg.Meta
		m.InstallMeta = &meta
		m.InstallMetaErr = ""
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
	case tea.KeyTab:
		if m.BusyAction || m.LoadingList {
			return m, nil
		}
		if len(m.Packages) == 0 || m.Selected < 0 || m.Selected >= len(m.Packages) {
			m.setLog(logInfo, "No selected package")
			return m, nil
		}
		name := strings.TrimSpace(m.Packages[m.Selected].Name)
		if name == "" {
			return m, nil
		}
		m.ModalOpen = true
		m.VersionModalOpen = true
		m.VersionFromMain = true
		m.VersionPackageName = name
		m.VersionLoading = true
		m.VersionsList = nil
		m.VersionSelected = 0
		m.VersionScroll = 0
		m.VersionErrorText = ""
		m.setLog(logLoading, fmt.Sprintf("Loading versions for %s...", name))
		return m, m.versionsCmd(name)
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
		case 's', 'S':
			m.openManagerModal()
			m.setLog(logInfo, "Select package manager")
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
	if msg.Type == tea.KeyTab {
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

		m.InstallMeta = nil
		m.InstallMetaErr = ""
		m.InstallMetaLoading = false

		m.VersionModalOpen = true
		m.VersionPackageName = pkgName
		m.VersionLoading = true
		m.VersionsList = nil
		m.VersionSelected = 0
		m.VersionScroll = 0
		m.VersionErrorText = ""
		m.setLog(logLoading, fmt.Sprintf("Loading versions for %s...", pkgName))
		return m, m.versionsCmd(pkgName)
	}

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

		m.InstallMeta = nil
		m.InstallMetaErr = ""
		m.InstallMetaLoading = false

		m.BusyAction = true
		m.ModalLoading = true
		m.ModalErrorText = ""
		m.setLog(logLoading, fmt.Sprintf("Installing %s...", pkgName))
		return m, m.installCmd(pkgName)
	case tea.KeyLeft:
		if m.ModalFocusedPane > 0 {
			m.ModalFocusedPane--
		}
		return m, nil
	case tea.KeyRight:
		if m.ModalFocusedPane < 1 {
			m.ModalFocusedPane++
		}
		return m, nil
	case tea.KeyUp, tea.KeyCtrlP:
		if m.SuggestionSelected > 0 {
			m.SuggestionSelected--
			m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
			m, cmd := m.beginInstallPackageMetaLoad(m.installSelectedPackageName())
			return m, cmd
		}
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if m.SuggestionSelected < len(m.Suggestions)-1 {
			m.SuggestionSelected++
			m.ensureSuggestionSelectionVisible(m.visibleSuggestionRows())
			m, cmd := m.beginInstallPackageMetaLoad(m.installSelectedPackageName())
			return m, cmd
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
		m, cmd := m.beginInstallPackageMetaLoad(m.installSelectedPackageName())
		return m, cmd
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
		m, cmd := m.beginInstallPackageMetaLoad(m.installSelectedPackageName())
		return m, cmd
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
		m.ModalSearchSeq = 0
		m.ModalErrorText = ""
		m.InstallMeta = nil
		m.InstallMetaErr = ""
		m.InstallMetaLoading = false
		return m, inputCmd
	}

	m.ModalLastQuery = after
	m.ModalLoadingSuggestions = true
	m.ModalErrorText = ""
	m.InstallMeta = nil
	m.InstallMetaErr = ""
	m.InstallMetaLoading = false
	searchCmd := m.searchSuggestionsCmd(after)
	if inputCmd == nil {
		return m, tea.Batch(searchCmd, m.Loader.Tick)
	}
	return m, tea.Batch(inputCmd, searchCmd, m.Loader.Tick)
}

func (m ViewModel) updateManagerModal(msg tea.KeyMsg) (ViewModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.closeModal()
		m.setLog(logInfo, "Manager selection closed")
		return m, nil
	case tea.KeyEnter:
		if len(m.ManagerOptions) == 0 {
			return m, nil
		}
		if m.ManagerSelected < 0 || m.ManagerSelected >= len(m.ManagerOptions) {
			return m, nil
		}
		selected := m.ManagerOptions[m.ManagerSelected]
		if !selected.Available {
			m.setLog(logInfo, selected.Label+" unavailable")
			return m, nil
		}

		switch selected.Key {
		case "uv":
			m.PackageManager = packagemanager.NewUVManager("uv")
		case "poetry":
			m.PackageManager = packagemanager.NewPoetryManager("poetry")
		default:
			m.PackageManager = packagemanager.NewPipManager("pip")
		}

		m.closeModal()
		m.setLog(logInfo, "Package manager selected: "+selected.Label)
		return m, nil
	case tea.KeyUp, tea.KeyCtrlP:
		m.ManagerSelected = m.prevAvailableManager(m.ManagerSelected)
		m.ensureManagerSelectionVisible(m.visibleManagerRows())
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		m.ManagerSelected = m.nextAvailableManager(m.ManagerSelected)
		m.ensureManagerSelectionVisible(m.visibleManagerRows())
		return m, nil
	case tea.KeyPgUp:
		step := m.visibleManagerRows()
		if step < 1 {
			step = 1
		}
		for i := 0; i < step; i++ {
			m.ManagerSelected = m.prevAvailableManager(m.ManagerSelected)
		}
		m.ensureManagerSelectionVisible(m.visibleManagerRows())
		return m, nil
	case tea.KeyPgDown:
		step := m.visibleManagerRows()
		if step < 1 {
			step = 1
		}
		for i := 0; i < step; i++ {
			m.ManagerSelected = m.nextAvailableManager(m.ManagerSelected)
		}
		m.ensureManagerSelectionVisible(m.visibleManagerRows())
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'k', 'K':
				m.ManagerSelected = m.prevAvailableManager(m.ManagerSelected)
				m.ensureManagerSelectionVisible(m.visibleManagerRows())
				return m, nil
			case 'j', 'J':
				m.ManagerSelected = m.nextAvailableManager(m.ManagerSelected)
				m.ensureManagerSelectionVisible(m.visibleManagerRows())
				return m, nil
			}
		}
	}

	return m, nil
}

func (m ViewModel) updateVersionModal(msg tea.KeyMsg) (ViewModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.VersionFromMain {
			m.closeModal()
		} else {
			m.VersionModalOpen = false
			m.VersionLoading = false
			m.VersionErrorText = ""
		}
		m.setLog(logInfo, "Version picker closed")
		return m, nil
	case tea.KeyEnter:
		if m.BusyAction || m.VersionLoading {
			return m, nil
		}
		pkgName := strings.TrimSpace(m.VersionPackageName)
		if pkgName == "" {
			m.VersionErrorText = "Package name is empty"
			return m, nil
		}
		if len(m.VersionsList) == 0 || m.VersionSelected < 0 || m.VersionSelected >= len(m.VersionsList) {
			m.VersionErrorText = "No version selected"
			return m, nil
		}

		selectedVersion := strings.TrimSpace(m.VersionsList[m.VersionSelected])
		if selectedVersion == "" {
			m.VersionErrorText = "No version selected"
			return m, nil
		}

		installTarget := fmt.Sprintf("%s==%s", pkgName, selectedVersion)
		m.BusyAction = true
		m.ModalLoading = true
		m.VersionErrorText = ""
		m.ActionModalOpen = true
		m.ActionModalLoading = true
		m.ActionModalTitle = "Installing Package"
		m.ActionModalText = fmt.Sprintf("Installing %s...", installTarget)
		m.ActionModalKind = logLoading
		m.setLog(logLoading, fmt.Sprintf("Installing %s...", installTarget))
		return m, m.installCmd(installTarget)
	case tea.KeyUp, tea.KeyCtrlP:
		if m.VersionSelected > 0 {
			m.VersionSelected--
			m.ensureVersionSelectionVisible(m.visibleVersionRows())
		}
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if m.VersionSelected < len(m.VersionsList)-1 {
			m.VersionSelected++
			m.ensureVersionSelectionVisible(m.visibleVersionRows())
		}
		return m, nil
	case tea.KeyPgUp:
		step := m.visibleVersionRows()
		if step < 1 {
			step = 1
		}
		m.VersionSelected -= step
		if m.VersionSelected < 0 {
			m.VersionSelected = 0
		}
		m.ensureVersionSelectionVisible(m.visibleVersionRows())
		return m, nil
	case tea.KeyPgDown:
		step := m.visibleVersionRows()
		if step < 1 {
			step = 1
		}
		m.VersionSelected += step
		if m.VersionSelected >= len(m.VersionsList) {
			m.VersionSelected = len(m.VersionsList) - 1
			if m.VersionSelected < 0 {
				m.VersionSelected = 0
			}
		}
		m.ensureVersionSelectionVisible(m.visibleVersionRows())
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'q', 'Q':
				if m.VersionFromMain {
					m.closeModal()
				} else {
					m.VersionModalOpen = false
					m.VersionLoading = false
					m.VersionErrorText = ""
				}
				m.setLog(logInfo, "Version picker closed")
				return m, nil
			case 'k', 'K':
				if m.VersionSelected > 0 {
					m.VersionSelected--
					m.ensureVersionSelectionVisible(m.visibleVersionRows())
				}
				return m, nil
			case 'j', 'J':
				if m.VersionSelected < len(m.VersionsList)-1 {
					m.VersionSelected++
					m.ensureVersionSelectionVisible(m.visibleVersionRows())
				}
				return m, nil
			}
		}
	}

	return m, nil
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
	if m.Width < minWidth || m.Height < minHeight {
		return m.renderInsufficientSpace()
	}

	helpHeight := 1
	bodyHeight := m.Height - helpHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	// Total rendered width is: left( +2 border ) + separator(1) + right( +2 border ).
	availableContentWidth := m.Width - 6
	if availableContentWidth < 2 {
		availableContentWidth = 2
	}
	leftWidth := availableContentWidth / 2
	rightWidth := availableContentWidth - leftWidth
	if leftWidth < 1 {
		leftWidth = 1
	}
	if rightWidth < 1 {
		rightWidth = 1
	}
	paneHeight := bodyHeight - 2
	if paneHeight < 1 {
		paneHeight = 1
	}

	basePaneStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	focusedPaneStyle := basePaneStyle.Bold(true).BorderForeground(reqTitleColor)

	leftStyle := basePaneStyle.Width(leftWidth).Height(paneHeight)
	rightStyle := basePaneStyle.Width(rightWidth).Height(paneHeight)
	if m.FocusedPane == 0 {
		leftStyle = focusedPaneStyle.Width(leftWidth).Height(paneHeight)
	} else {
		rightStyle = focusedPaneStyle.Width(rightWidth).Height(paneHeight)
	}

	listRows := bodyHeight - 6
	if listRows < 1 {
		listRows = 1
	}

	leftPane := leftStyle.Render(m.renderInstalledPackages(max(1, leftWidth), listRows))
	if len(m.Packages) > listRows {
		leftPane = overlayScrollbarOnBorder(leftPane, len(m.Packages), m.Scroll, listRows)
	}

	rightRows := bodyHeight - 4
	if rightRows < 1 {
		rightRows = 1
	}
	detailContent, detailTotal, detailScroll := m.renderPackageDetails(max(1, rightWidth), rightRows)
	rightPane := rightStyle.Render(detailContent)
	if detailTotal > rightRows {
		rightPane = overlayScrollbarOnBorder(rightPane, detailTotal, detailScroll, rightRows)
	}

	mainPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane)

	helpLine := m.renderBottomHelp()
	baseMain := mainPanes
	if m.ModalOpen && !m.VersionFromMain {
		installView := m.renderInstallModal()
		installBase := installView

		if m.ActionModalOpen && m.ModalLoading {
			actionModal := m.renderActionModal()
			actionX, actionY := centeredOverlayPosition(actionModal, m.Width, m.Height)
			installBase = overlayAt(stripANSI(installView), actionModal, actionX, actionY)
		} else if m.VersionModalOpen {
			versionModal := m.renderVersionModal()
			versionX, versionY := centeredOverlayPosition(versionModal, m.Width, m.Height)
			installBase = overlayAt(stripANSI(installView), versionModal, versionX, versionY)
		}

		return lipgloss.JoinVertical(lipgloss.Left, installBase, helpLine)
	}

	if !m.ModalOpen && !m.ActionModalOpen && !m.HelpModalOpen {
		return lipgloss.JoinVertical(lipgloss.Left, baseMain, helpLine)
	}

	modalBase := stripANSI(baseMain)
	if m.ModalOpen {
		if m.ManagerModalOpen {
			managerModal := m.renderManagerModal()
			managerX, managerY := centeredOverlayPosition(managerModal, m.Width, m.Height)
			modalBase = overlayAt(modalBase, managerModal, managerX, managerY)
		} else if !m.VersionFromMain {
			installModal := m.renderInstallModal()
			installX, installY := centeredOverlayPosition(installModal, m.Width, m.Height)
			modalBase = overlayAt(modalBase, installModal, installX, installY)
		}
		if m.VersionModalOpen && !m.ManagerModalOpen {
			versionModal := m.renderVersionModal()
			versionX, versionY := centeredOverlayPosition(versionModal, m.Width, m.Height)
			modalBase = overlayAt(modalBase, versionModal, versionX, versionY)
		}
	} else if m.HelpModalOpen {
		helpModal := m.renderHelpModal()
		helpX, helpY := centeredOverlayPosition(helpModal, m.Width, m.Height)
		modalBase = overlayAt(modalBase, helpModal, helpX, helpY)
	} else {
		actionModal := m.renderActionModal()
		actionX, actionY := centeredOverlayPosition(actionModal, m.Width, m.Height)
		modalBase = overlayAt(modalBase, actionModal, actionX, actionY)
	}

	return lipgloss.JoinVertical(lipgloss.Left, modalBase, helpLine)
}

func (m ViewModel) renderInsufficientSpace() string {
	message := strings.Join([]string{
		"Not enough terminal space",
		fmt.Sprintf("Current: %dx%d", m.Width, m.Height),
		fmt.Sprintf("Minimum: %dx%d", minWidth, minHeight),
		"Resize the terminal to continue.",
	}, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(message)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, box)
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

		baseRunes := []rune(stripANSI(baseLines[target]))
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

func centeredOverlayPosition(content string, width int, height int) (int, int) {
	modalWidth := lipgloss.Width(stripANSI(content))
	modalHeight := lipgloss.Height(stripANSI(content))
	x := (width - modalWidth) / 2
	if x < 0 {
		x = 0
	}
	y := (height - modalHeight) / 2
	if y < 0 {
		y = 0
	}
	return x, y
}

func commandAvailable(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	_, err := exec.LookPath(name)
	return err == nil
}

func (m *ViewModel) openModal() {
	m.ModalOpen = true
	m.ManagerModalOpen = false
	m.InstallInput.SetValue("")
	m.InstallInput.Focus()
	m.Suggestions = nil
	m.SuggestionSelected = 0
	m.SuggestionScroll = 0
	m.ModalLastQuery = ""
	m.ModalSearchSeq = 0
	m.ModalLoadingSuggestions = false
	m.InstallMeta = nil
	m.InstallMetaLoading = false
	m.InstallMetaErr = ""
}

func (m *ViewModel) openManagerModal() {
	m.ModalOpen = true
	m.ManagerModalOpen = true
	m.InstallInput.Blur()
	m.InstallInput.SetValue("")
	m.Suggestions = nil
	m.SuggestionSelected = 0
	m.SuggestionScroll = 0
	m.ModalLastQuery = ""
	m.ModalLoadingSuggestions = false
	m.ModalErrorText = ""
	m.ModalLoading = false
	m.VersionModalOpen = false
	m.VersionLoading = false
	m.VersionErrorText = ""

	m.ManagerOptions = []managerOption{
		{Key: "pip", Label: "pip", Available: commandAvailable("pip")},
		{Key: "uv", Label: "uv", Available: commandAvailable("uv")},
		{Key: "poetry", Label: "poetry", Available: commandAvailable("poetry")},
	}
	currentKey := m.currentManagerKey()
	m.ManagerSelected = m.firstAvailableManager()
	for i, option := range m.ManagerOptions {
		if option.Key == currentKey {
			m.ManagerSelected = i
			break
		}
	}
	m.ManagerScroll = 0
	m.ensureManagerSelectionVisible(m.visibleManagerRows())
}

func (m ViewModel) currentManagerKey() string {
	switch m.PackageManager.(type) {
	case *packagemanager.UVManager:
		return "uv"
	case *packagemanager.PoetryManager:
		return "poetry"
	default:
		return "pip"
	}
}

func (m *ViewModel) closeModal() {
	m.ModalOpen = false
	m.InstallInput.Blur()
	m.InstallInput.SetValue("")
	m.Suggestions = nil
	m.SuggestionSelected = 0
	m.SuggestionScroll = 0
	m.ModalLastQuery = ""
	m.ModalSearchSeq = 0
	m.ModalLoadingSuggestions = false
	m.InstallMeta = nil
	m.InstallMetaLoading = false
	m.InstallMetaErr = ""
	m.ModalErrorText = ""
	m.ModalLoading = false
	m.ManagerModalOpen = false
	m.ManagerOptions = nil
	m.ManagerSelected = 0
	m.ManagerScroll = 0
	m.VersionModalOpen = false
	m.VersionFromMain = false
	m.VersionsList = nil
	m.VersionSelected = 0
	m.VersionScroll = 0
	m.VersionLoading = false
	m.VersionPackageName = ""
	m.VersionErrorText = ""
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

func (m ViewModel) beginInstallPackageMetaLoad(name string) (ViewModel, tea.Cmd) {
	name = strings.TrimSpace(name)
	if name == "" {
		m.InstallMeta = nil
		m.InstallMetaErr = ""
		m.InstallMetaLoading = false
		return m, nil
	}

	cacheKey := strings.ToLower(name)
	if cached, ok := m.metaCache[cacheKey]; ok {
		meta := cached
		m.InstallMeta = &meta
		m.InstallMetaErr = ""
		m.InstallMetaLoading = false
		return m, nil
	}

	m.InstallMeta = nil
	m.InstallMetaErr = ""
	m.InstallMetaLoading = true

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
			return installPackageMetaLoadedMsg{Name: name, Err: ctx.Err()}
		case res := <-ch:
			return installPackageMetaLoadedMsg{Name: name, Meta: res.meta, Err: res.err}
		}
	}

	return m, tea.Batch(cmd, m.Loader.Tick)
}

func (m ViewModel) loadingLine(text string, width int) string {
	if width < 1 {
		width = 1
	}
	line := strings.TrimSpace(m.Loader.View() + " " + strings.TrimSpace(text))
	return lipgloss.NewStyle().Foreground(reqVenvColor).Render(TruncateText(line, width))
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

func (m ViewModel) versionsCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		versions, err := m.PackageManager.Versions(ctx, name)
		return versionsDoneMsg{Name: name, Versions: versions, Err: err}
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

	return m, tea.Batch(cmd, m.Loader.Tick)
}

func (m ViewModel) beginModalSuggestionMetaLoad() (ViewModel, tea.Cmd) {
	var name string
	if len(m.Suggestions) > 0 && m.SuggestionSelected >= 0 && m.SuggestionSelected < len(m.Suggestions) {
		name = strings.TrimSpace(m.Suggestions[m.SuggestionSelected].Name)
	}
	if name == "" {
		m.ModalSuggestionMeta = nil
		m.ModalSuggestionMetaLoading = false
		m.ModalSuggestionMetaName = ""
		m.ModalSuggestionMetaScroll = 0
		return m, nil
	}
	if m.ModalSuggestionMetaName == name {
		return m, nil
	}
	m.ModalSuggestionMetaScroll = 0
	cacheKey := strings.ToLower(name)
	if cached, ok := m.metaCache[cacheKey]; ok {
		meta := cached
		m.ModalSuggestionMeta = &meta
		m.ModalSuggestionMetaLoading = false
		m.ModalSuggestionMetaName = name
		return m, nil
	}
	m.ModalSuggestionMeta = nil
	m.ModalSuggestionMetaLoading = true
	m.ModalSuggestionMetaName = name
	cmd := func() tea.Msg {
		meta, err := fetchPackageMetadata(name)
		return modalSuggestionMetaLoadedMsg{Name: name, Meta: meta, Err: err}
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

func (m *ViewModel) ensureVersionSelectionVisible(visibleRows int) {
	if visibleRows < 1 {
		visibleRows = 1
	}
	if m.VersionSelected < m.VersionScroll {
		m.VersionScroll = m.VersionSelected
	}
	if m.VersionSelected >= m.VersionScroll+visibleRows {
		m.VersionScroll = m.VersionSelected - visibleRows + 1
	}
	if m.VersionScroll < 0 {
		m.VersionScroll = 0
	}
	maxScroll := len(m.VersionsList) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.VersionScroll > maxScroll {
		m.VersionScroll = maxScroll
	}
}

func (m *ViewModel) ensureManagerSelectionVisible(visibleRows int) {
	if visibleRows < 1 {
		visibleRows = 1
	}
	if m.ManagerSelected < m.ManagerScroll {
		m.ManagerScroll = m.ManagerSelected
	}
	if m.ManagerSelected >= m.ManagerScroll+visibleRows {
		m.ManagerScroll = m.ManagerSelected - visibleRows + 1
	}
	if m.ManagerScroll < 0 {
		m.ManagerScroll = 0
	}
	maxScroll := len(m.ManagerOptions) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.ManagerScroll > maxScroll {
		m.ManagerScroll = maxScroll
	}
}

func (m ViewModel) firstAvailableManager() int {
	for i, option := range m.ManagerOptions {
		if option.Available {
			return i
		}
	}
	if len(m.ManagerOptions) == 0 {
		return 0
	}
	return 0
}

func (m ViewModel) prevAvailableManager(from int) int {
	if len(m.ManagerOptions) == 0 {
		return 0
	}
	start := from
	if start < 0 || start >= len(m.ManagerOptions) {
		start = m.firstAvailableManager()
	}
	idx := start
	for i := 0; i < len(m.ManagerOptions); i++ {
		idx--
		if idx < 0 {
			idx = len(m.ManagerOptions) - 1
		}
		if m.ManagerOptions[idx].Available {
			return idx
		}
	}
	return start
}

func (m ViewModel) nextAvailableManager(from int) int {
	if len(m.ManagerOptions) == 0 {
		return 0
	}
	start := from
	if start < 0 || start >= len(m.ManagerOptions) {
		start = m.firstAvailableManager()
	}
	idx := start
	for i := 0; i < len(m.ManagerOptions); i++ {
		idx++
		if idx >= len(m.ManagerOptions) {
			idx = 0
		}
		if m.ManagerOptions[idx].Available {
			return idx
		}
	}
	return start
}

func (m ViewModel) visibleMainRows() int {
	rows := m.Height - 7
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m ViewModel) visibleSuggestionRows() int {
	modalHeight := int(float64(m.Height) * 0.8)
	maxInstallModalHeight := m.Height - 4
	if maxInstallModalHeight > 34 {
		maxInstallModalHeight = 34
	}
	if maxInstallModalHeight < 3 {
		maxInstallModalHeight = 3
	}
	if modalHeight > maxInstallModalHeight {
		modalHeight = maxInstallModalHeight
	}
	if modalHeight < 3 {
		modalHeight = 3
	}

	innerHeight := modalHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}
	contentHeight := innerHeight - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	rows := contentHeight - 3 // title + input + subtitle in left pane
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m ViewModel) visibleVersionRows() int {
	rows := int(float64(m.Height) * 0.45)
	rows -= 9
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m ViewModel) visibleManagerRows() int {
	rows := int(float64(m.Height) * 0.22)
	rows -= 6
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m ViewModel) renderInstalledPackages(width int, rows int) string {
	if width < 1 {
		width = 1
	}
	if rows < 1 {
		rows = 1
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Installed Packages")
	loadingStyle := lipgloss.NewStyle().Foreground(reqVenvColor)
	emptyStyle := lipgloss.NewStyle().Foreground(reqMutedColor)
	mutedStyle := lipgloss.NewStyle().Foreground(reqMutedColor)
	nameStyle := lipgloss.NewStyle().Foreground(reqGlobalColor)
	versionStyle := lipgloss.NewStyle().Foreground(reqMutedColor)

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

	// compute aligned name column width from visible slice
	rowTextWidth := width - 2 // 2 for "> " / "  " prefix
	maxName := 0
	for i := start; i < end; i++ {
		if w := lipgloss.Width(m.Packages[i].Name); w > maxName {
			maxName = w
		}
	}
	capWidth := max(8, rowTextWidth-14)
	nameWidth := maxName
	if nameWidth > capWidth {
		nameWidth = capWidth
	}
	if nameWidth < 8 {
		nameWidth = 8
	}

	lines := []string{header, ""}
	pkgLines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		dep := m.Packages[i]
		selected := i == m.Selected

		name := dep.Name
		if lipgloss.Width(name) > nameWidth {
			name = TruncateText(name, nameWidth)
		}
		pad := nameWidth - lipgloss.Width(name)
		if pad < 0 {
			pad = 0
		}
		nameCol := nameStyle.Render(name + strings.Repeat(" ", pad))
		verCol := versionStyle.Render(dep.Version)
		row := TruncateText(lipgloss.JoinHorizontal(lipgloss.Top, nameCol, "  ", verCol), rowTextWidth)

		if selected {
			row = lipgloss.NewStyle().Reverse(true).Bold(true).Render("> " + row)
		} else {
			row = "  " + row
		}
		pkgLines = append(pkgLines, row)
	}

	lines = append(lines, pkgLines...)
	lines = append(lines, "", mutedStyle.Render(fmt.Sprintf("%d/%d", m.Selected+1, len(m.Packages))))
	return strings.Join(lines, "\n")
}

func (m ViewModel) renderPackageDetails(width int, rows int) (string, int, int) {
	if width < 1 {
		width = 1
	}
	if rows < 1 {
		rows = 1
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
		lines = append(lines, "", m.loadingLine("Loading README from PyPI...", width))
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
			readmePreview, truncated := readmeMarkdownPreview(m.SelectedMeta.Readme)
			lines = append(lines, "", lipgloss.NewStyle().Foreground(reqKeyColor).Render("README"), m.renderMarkdownWithGlamour(readmePreview, width))
			if truncated {
				lines = append(lines, muted.Render("README preview truncated for performance."))
			}
		}
	}

	return m.renderScrollableDetails(lines, width, rows)
}

func (m ViewModel) renderScrollableDetails(lines []string, width int, rows int) (string, int, int) {
	flat := make([]string, 0, len(lines))
	for _, line := range lines {
		flat = append(flat, strings.Split(line, "\n")...)
	}
	for i := range flat {
		flat[i] = clampLineToWidth(flat[i], width)
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

	out := normalizeViewportLines(flat[start:end], width, rows)

	return strings.Join(out, "\n"), len(flat), start
}

func (m ViewModel) renderMarkdownWithGlamour(markdown string, width int) string {
	md := strings.TrimSpace(markdown)
	if md == "" {
		return ""
	}
	if width < 8 {
		width = 8
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
	md = markdownImagePattern.ReplaceAllString(md, "")
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
	if width < 8 {
		width = 8
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

func readmeMarkdownPreview(markdown string) (string, bool) {
	md := strings.TrimSpace(markdown)
	if md == "" {
		return "", false
	}
	if len(md) <= readmePreviewMaxChars {
		return md, false
	}

	cutoff := strings.LastIndex(md[:readmePreviewMaxChars], "\n")
	if cutoff < readmePreviewMaxChars/2 {
		cutoff = readmePreviewMaxChars
	}
	return strings.TrimSpace(md[:cutoff]), true
}

func (m ViewModel) renderInstallModal() string {
	modalWidth := m.Width - 2
	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalWidth%2 == 0 && modalWidth > 40 {
		modalWidth--
	}

	modalHeight := m.Height - 2
	if modalHeight < 10 {
		modalHeight = 10
	}
	if modalHeight > m.Height-1 {
		modalHeight = m.Height - 1
	}
	innerHeight := modalHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	panelWidth := (modalWidth - 1) / 2
	if panelWidth < 20 {
		panelWidth = 20
	}
	panelHeight := innerHeight
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(reqTitleColor).
		Width(modalWidth).
		Height(modalHeight)
	leftPanel := m.renderInstallSuggestionsPanel(panelWidth, panelHeight)
	rightPanel := m.renderInstallPackageInfoPanel(panelWidth, panelHeight)
	separator := lipgloss.NewStyle().Foreground(reqMutedColor).Width(1).Render("|")
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, separator, rightPanel)
	return modalStyle.Render(body)
}

func (m ViewModel) renderInstallSuggestionsPanel(width int, rows int) string {
	if width < 20 {
		width = 20
	}
	if rows < 4 {
		rows = 4
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Install Package")
	muted := lipgloss.NewStyle().Foreground(reqMutedColor)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(reqVenvColor).Reverse(true)
	inputStyle := lipgloss.NewStyle().Foreground(reqValueColor).Bold(true)

	lines := []string{title, inputStyle.Render(m.InstallInput.View()), ""}
	remainingRows := rows - len(lines)
	if remainingRows < 1 {
		remainingRows = 1
	}

	start := m.SuggestionScroll
	if start < 0 {
		start = 0
	}
	if start >= len(m.Suggestions) && len(m.Suggestions) > 0 {
		start = len(m.Suggestions) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + remainingRows
	if end > len(m.Suggestions) {
		end = len(m.Suggestions)
	}

	if m.ModalLoadingSuggestions {
		lines = append(lines, m.loadingLine("Searching suggestions...", width))
	} else if len(m.Suggestions) == 0 {
		lines = append(lines, muted.Render("No suggestions"))
	} else {
		for i := start; i < end; i++ {
			line := TruncateText(m.Suggestions[i].Name, width-4)
			if i == m.SuggestionSelected {
				line = selectedStyle.Render(line)
			}
			lines = append(lines, line)
		}
	}

	if m.ModalLoading {
		lines = append(lines, muted.Render("Installing..."))
	} else if m.ModalErrorText != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(reqVersionColor).Render(WrapText(m.ModalErrorText, width-2)))
	}

	panelStyle := lipgloss.NewStyle().Width(width).Height(rows)
	return panelStyle.Render(renderFixedLines(lines, width, rows))
}

func (m ViewModel) renderInstallPackageInfoPanel(width int, rows int) string {
	if width < 18 {
		width = 18
	}
	if rows < 4 {
		rows = 4
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Package Info")
	labelStyle := lipgloss.NewStyle().Foreground(reqKeyColor)
	valueStyle := lipgloss.NewStyle().Foreground(reqValueColor).Bold(true)
	muted := lipgloss.NewStyle().Foreground(reqMutedColor)
	warning := lipgloss.NewStyle().Foreground(reqVersionColor)

	name := m.installSelectedPackageName()
	version := "unknown"
	if len(m.Suggestions) > 0 && m.SuggestionSelected >= 0 && m.SuggestionSelected < len(m.Suggestions) {
		selected := m.Suggestions[m.SuggestionSelected]
		if strings.TrimSpace(selected.Name) != "" {
			name = strings.TrimSpace(selected.Name)
		}
		if strings.TrimSpace(selected.Version) != "" {
			version = strings.TrimSpace(selected.Version)
		}
	}
	if m.InstallMeta != nil && strings.EqualFold(strings.TrimSpace(m.InstallMeta.Name), strings.TrimSpace(name)) {
		if strings.TrimSpace(m.InstallMeta.Version) != "" {
			version = strings.TrimSpace(m.InstallMeta.Version)
		}
	}

	lines := []string{title, ""}
	if strings.TrimSpace(name) == "" {
		lines = append(lines, muted.Render("Type a package name or pick a suggestion."))
		panelStyle := lipgloss.NewStyle().Width(width).Height(rows)
		return panelStyle.Render(renderFixedLines(lines, width, rows))
	}

	lines = append(lines,
		labelStyle.Render("Name"),
		valueStyle.Render(TruncateText(name, width-2)),
		"",
		labelStyle.Render("Latest version"),
		valueStyle.Render(TruncateText(version, width-2)),
		"",
		labelStyle.Render("Description"),
	)

	if m.InstallMetaLoading {
		lines = append(lines, m.loadingLine("Loading description from PyPI...", width-2))
	} else if m.InstallMetaErr != "" {
		lines = append(lines, warning.Render(WrapText(m.InstallMetaErr, width-2)))
	} else if m.InstallMeta != nil && strings.EqualFold(strings.TrimSpace(m.InstallMeta.Name), strings.TrimSpace(name)) {
		description := strings.TrimSpace(m.InstallMeta.Description)
		if description == "" {
			description = "No description available."
		}
		lines = append(lines, WrapText(description, width-2))
	} else {
		lines = append(lines, muted.Render("Loading package details..."))
	}

	panelStyle := lipgloss.NewStyle().Width(width).Height(rows)
	return panelStyle.Render(renderFixedLines(lines, width, rows))
}

func renderFixedLines(lines []string, width int, rows int) string {
	if width < 1 {
		width = 1
	}
	if rows < 1 {
		rows = 1
	}

	flat := make([]string, 0, len(lines))
	for _, line := range lines {
		flat = append(flat, strings.Split(line, "\n")...)
	}

	flat = normalizeViewportLines(flat, width, rows)

	return strings.Join(flat, "\n")
}

func normalizeViewportLines(lines []string, width int, rows int) []string {
	if width < 1 {
		width = 1
	}
	if rows < 1 {
		rows = 1
	}

	clamped := make([]string, 0, len(lines))
	for _, line := range lines {
		clamped = append(clamped, clampLineToWidth(line, width))
	}

	if len(clamped) > rows {
		clamped = clamped[:rows]
	} else {
		for len(clamped) < rows {
			clamped = append(clamped, "")
		}
	}

	return clamped
}

func clampLineToWidth(line string, width int) string {
	if width < 1 {
		return ""
	}
	visible := stripANSI(line)
	if lipgloss.Width(visible) <= width {
		return line
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

func (m ViewModel) installSelectedPackageName() string {
	if len(m.Suggestions) > 0 && m.SuggestionSelected >= 0 && m.SuggestionSelected < len(m.Suggestions) {
		candidate := strings.TrimSpace(m.Suggestions[m.SuggestionSelected].Name)
		if candidate != "" {
			return candidate
		}
	}
	return strings.TrimSpace(m.InstallInput.Value())
}

func (m ViewModel) renderManagerModal() string {
	modalWidth := int(float64(m.Width) * 0.16)
	if modalWidth < 20 {
		modalWidth = 20
	}
	if modalWidth > m.Width-2 {
		modalWidth = m.Width - 2
	}

	modalHeight := int(float64(m.Height) * 0.22)
	if modalHeight < 6 {
		modalHeight = 6
	}
	if modalHeight > m.Height-2 {
		modalHeight = m.Height - 2
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(reqTitleColor).
		Width(modalWidth).
		Height(modalHeight)

	header := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Select Package Manager")
	muted := lipgloss.NewStyle().Foreground(reqMutedColor)
	unavailableStyle := lipgloss.NewStyle().Foreground(reqMutedColor)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(reqVenvColor).Reverse(true)

	rows := m.visibleManagerRows()
	start := m.ManagerScroll
	if start < 0 {
		start = 0
	}
	if start >= len(m.ManagerOptions) {
		start = len(m.ManagerOptions) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + rows
	if end > len(m.ManagerOptions) {
		end = len(m.ManagerOptions)
	}

	managerLines := make([]string, 0, rows)
	currentKey := m.currentManagerKey()
	if len(m.ManagerOptions) == 0 {
		managerLines = append(managerLines, muted.Render("No package managers detected"))
	} else {
		for i := start; i < end; i++ {
			option := m.ManagerOptions[i]
			line := option.Label
			if option.Key == currentKey {
				line += " (selected)"
			}
			if !option.Available {
				line += "  unavailable"
			}
			prefix := "  "
			if i == m.ManagerSelected {
				prefix = "> "
			}
			line = TruncateText(prefix+line, modalWidth-8)
			if option.Available && i == m.ManagerSelected {
				line = selectedStyle.Render(line)
			} else if !option.Available {
				line = unavailableStyle.Render(line)
			}
			managerLines = append(managerLines, line)
		}
	}

	body := []string{header, ""}
	body = append(body, managerLines...)

	innerHeight := modalHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}
	body = normalizeViewportLines(body, modalWidth-2, innerHeight)

	return modalStyle.Render(strings.Join(body, "\n"))
}

func (m ViewModel) renderVersionModal() string {
	// Calculate width based on content: longest version + package name + header + padding
	longestVersion := 10 // default minimum
	for _, v := range m.VersionsList {
		if len(v) > longestVersion {
			longestVersion = len(v)
		}
	}
	pkgNameLen := len(strings.TrimSpace(m.VersionPackageName))
	headerLen := len("Select Version")

	// Calculate needed width: max of (header, package line, version list)
	neededWidth := max(headerLen, pkgNameLen+10) // +10 for "Package: " prefix
	neededWidth = max(neededWidth, longestVersion)
	neededWidth += 4 // left/right padding and borders

	modalWidth := neededWidth
	if modalWidth < 16 {
		modalWidth = 16
	}
	if modalWidth > int(float64(m.Width)*0.5) {
		modalWidth = int(float64(m.Width) * 0.5) // max 50% of screen
	}
	maxVersionModalWidth := m.Width - 2
	if maxVersionModalWidth < 3 {
		maxVersionModalWidth = 3
	}
	if modalWidth > maxVersionModalWidth {
		modalWidth = maxVersionModalWidth
	}

	modalHeight := int(float64(m.Height) * 0.40)
	maxVersionModalHeight := m.Height - 4
	if maxVersionModalHeight > 20 {
		maxVersionModalHeight = 20
	}
	if maxVersionModalHeight < 3 {
		maxVersionModalHeight = 3
	}
	if modalHeight > maxVersionModalHeight {
		modalHeight = maxVersionModalHeight
	}
	if modalHeight < 3 {
		modalHeight = 3
	}

	if !m.VersionFromMain {
		modalWidth = m.Width - 2
		if modalWidth < 16 {
			modalWidth = 16
		}
		modalHeight = m.Height - 2
		if modalHeight < 3 {
			modalHeight = 3
		}
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(reqTitleColor).
		Width(modalWidth).
		Height(modalHeight)

	header := lipgloss.NewStyle().Bold(true).Foreground(reqTitleColor).Render("Select Version")

	pkgName := TruncateText(strings.TrimSpace(m.VersionPackageName), max(8, modalWidth-16))
	packageLine := lipgloss.NewStyle().
		Bold(true).
		Foreground(reqGlobalColor).
		Background(lipgloss.Color("236")).
		Padding(0, 1).
		Render("Package: " + pkgName)

	muted := lipgloss.NewStyle().Foreground(reqMutedColor)
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(reqVenvColor).Reverse(true)

	innerHeight := modalHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	listContentWidth := modalWidth - 4
	if listContentWidth < 8 {
		listContentWidth = 8
	}

	// ---- error / loading block ----
	errorBlock := ""
	errorLinesCount := 0
	if m.VersionErrorText != "" {
		wrapped := WrapText(m.VersionErrorText, modalWidth-8)
		errorBlock = lipgloss.NewStyle().Foreground(reqVersionColor).Render(wrapped)
		errorLinesCount = len(strings.Split(wrapped, "\n"))
	} else if m.ModalLoading {
		errorBlock = lipgloss.NewStyle().Foreground(reqVenvColor).Render("Installing selected version...")
		errorLinesCount = 1
	}

	// ---- status line ----
	statusLine := ""
	if len(m.VersionsList) > 0 && !m.VersionLoading {
		statusLine = muted.Render(
			TruncateText(fmt.Sprintf("%d/%d", m.VersionSelected+1, len(m.VersionsList)), modalWidth-8),
		)
	}

	// ---- compute available rows for list ----
	reserved := 2 // header + package

	if statusLine != "" {
		reserved++
	}
	reserved += errorLinesCount

	listVisibleRows := innerHeight - reserved
	if listVisibleRows < 1 {
		listVisibleRows = 1
	}

	// ---- scrolling window ----
	start := m.VersionScroll
	if start < 0 {
		start = 0
	}
	maxStart := len(m.VersionsList) - listVisibleRows
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	if start >= len(m.VersionsList) {
		start = len(m.VersionsList) - 1
	}
	if start < 0 {
		start = 0
	}

	end := start + listVisibleRows
	if end > len(m.VersionsList) {
		end = len(m.VersionsList)
	}

	// ---- build list ----
	versionLines := make([]string, 0, listVisibleRows)

	if m.VersionLoading {
		versionLines = append(versionLines,
			lipgloss.NewStyle().Foreground(reqVenvColor).Render(" "+TruncateText("Loading versions...", max(1, listContentWidth-1))),
		)
	} else if len(m.VersionsList) == 0 {
		versionLines = append(versionLines, muted.Render(" "+TruncateText("No versions found", max(1, listContentWidth-1))))
	} else {
		for i := start; i < end; i++ {
			line := " " + TruncateText(m.VersionsList[i], max(1, listContentWidth-1))
			if i == m.VersionSelected {
				line = selectedStyle.Render(line)
			}
			versionLines = append(versionLines, line)
		}
	}

	for len(versionLines) < listVisibleRows {
		versionLines = append(versionLines, "")
	}

	if len(m.VersionsList) > listVisibleRows && !m.VersionLoading {
		thumbHeight := listVisibleRows * listVisibleRows / len(m.VersionsList)
		if thumbHeight < 1 {
			thumbHeight = 1
		}
		maxScroll := len(m.VersionsList) - listVisibleRows
		thumbPos := 0
		if maxScroll > 0 {
			thumbPos = start * (listVisibleRows - thumbHeight) / maxScroll
		}
		trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
		thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
		textWidth := listContentWidth - 2
		if textWidth < 1 {
			textWidth = 1
		}
		for i := 0; i < len(versionLines); i++ {
			line := TruncateText(versionLines[i], textWidth)
			pad := textWidth - lipgloss.Width(stripANSI(line))
			if pad < 0 {
				pad = 0
			}
			ch := trackStyle.Render("│")
			if i >= thumbPos && i < thumbPos+thumbHeight {
				ch = thumbStyle.Render("█")
			}
			versionLines[i] = line + strings.Repeat(" ", pad) + ch
		}
	}

	if len(m.VersionsList) > 0 && !m.VersionLoading {
		statusLine = muted.Render(TruncateText(fmt.Sprintf("%d/%d", m.VersionSelected+1, len(m.VersionsList)), modalWidth-8))
	}

	body := []string{header, packageLine}
	body = append(body, versionLines...)
	body = append(body, statusLine)
	if errorBlock != "" {
		body = append(body, strings.Split(errorBlock, "\n")...)
	}

	flat := make([]string, 0, innerHeight)
	for _, line := range body {
		flat = append(flat, strings.Split(line, "\n")...)
	}
	flat = normalizeViewportLines(flat, modalWidth-2, innerHeight)

	return modalStyle.Render(strings.Join(flat, "\n"))
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
	lines = normalizeViewportLines(lines, modalWidth-2, innerHeight)

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
		key.Render("i") + detail.Render("  choose manager and install package"),
		key.Render("s") + detail.Render("  choose package manager"),
		key.Render("Tab (install modal)") + detail.Render("  open versions list"),
		key.Render("d") + detail.Render("  uninstall selected package"),
		key.Render("j/k or ↑/↓") + detail.Render("  move in package list"),
		key.Render("PgUp/PgDown") + detail.Render("  jump in package list"),
		key.Render("u/U") + detail.Render("  scroll details/readme"),
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
	innerWidth := modalWidth - 6
	if innerWidth < 1 {
		innerWidth = 1
	}
	innerHeight := modalHeight - 4
	if innerHeight < 1 {
		innerHeight = 1
	}
	rows = normalizeViewportLines(rows, innerWidth, innerHeight)

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
		if m.ManagerModalOpen {
			legend := lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render("Enter"), sepStyle.Render(": choose manager"),
				sepStyle.Render("  |  "),
				keyStyle.Render("j/k or PgUp/PgDown"), sepStyle.Render(": move"),
				sepStyle.Render("  |  "),
				keyStyle.Render("Esc"), sepStyle.Render(": close"),
			)
			return TruncateText(legend, m.Width)
		}

		if m.VersionModalOpen {
			legend := lipgloss.JoinHorizontal(lipgloss.Top,
				keyStyle.Render("Enter"), sepStyle.Render(": install selected version"),
				sepStyle.Render("  |  "),
				keyStyle.Render("j/k, ↑/↓, PgUp/PgDown"), sepStyle.Render(": move"),
				sepStyle.Render("  |  "),
				keyStyle.Render("Esc"), sepStyle.Render(": back"),
				sepStyle.Render("  |  "),
				keyStyle.Render("q"), sepStyle.Render(": back"),
			)
			return TruncateText(legend, m.Width)
		}

		legend := lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render("Enter"), sepStyle.Render(": install"),
			sepStyle.Render("  |  "),
			keyStyle.Render("←/→"), sepStyle.Render(": focus"),
			sepStyle.Render("  |  "),
			keyStyle.Render("↑/↓, PgUp/PgDown"), sepStyle.Render(": move/scroll"),
			sepStyle.Render("  |  "),
			keyStyle.Render("Tab"), sepStyle.Render(": versions"),
			sepStyle.Render("  |  "),
			keyStyle.Render("Esc"), sepStyle.Render(": back"),
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
			keyStyle.Render("Enter"), sepStyle.Render(": close"),
			sepStyle.Render("  |  "),
			keyStyle.Render("Esc"), sepStyle.Render(": close"),
		)
		return TruncateText(legend, m.Width)
	}

	if m.HelpModalOpen {
		legend := lipgloss.JoinHorizontal(lipgloss.Top,
			keyStyle.Render("Enter"), sepStyle.Render(": close"),
			sepStyle.Render("  |  "),
			keyStyle.Render("?"), sepStyle.Render(": close"),
			sepStyle.Render("  |  "),
			keyStyle.Render("Esc"), sepStyle.Render(": close"),
		)
		return TruncateText(legend, m.Width)
	}

	leftLegend := lipgloss.JoinHorizontal(lipgloss.Top,
		keyStyle.Render("i"), sepStyle.Render(": install"),
		sepStyle.Render("  |  "),
		keyStyle.Render("s"), sepStyle.Render(": manager"),
		sepStyle.Render("  |  "),
		keyStyle.Render("d"), sepStyle.Render(": uninstall"),
		sepStyle.Render("  |  "),
		keyStyle.Render("Tab"), sepStyle.Render(": versions"),
		sepStyle.Render("  |  "),
		keyStyle.Render("?"), sepStyle.Render(": more"),
		sepStyle.Render("  |  "),
		keyStyle.Render("Esc"), sepStyle.Render(": menu"),
		sepStyle.Render("  |  "),
		keyStyle.Render("q"), sepStyle.Render(": quit"),
	)
	rightLegend := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Foreground(reqValueColor).Render("package manager:"),
		lipgloss.NewStyle().Render(" "),
		lipgloss.NewStyle().Foreground(reqKeyColor).Render(m.currentManagerKey()),
	)
	spacer := lipgloss.NewStyle().Width(max(0, m.Width-lipgloss.Width(leftLegend)-lipgloss.Width(rightLegend))).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, leftLegend, spacer, rightLegend)
}
