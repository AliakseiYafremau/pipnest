package venvs

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// BackMsg is sent when the user wants to return to the main menu.
type BackMsg struct{}

const (
	minWidth  = 72
	minHeight = 14
)

type replFinishedMsg struct {
	err error
}

type detailsLoadedMsg struct {
	path    string
	details InterpreterDetails
	err     error
}

// Model holds the full state of the venvs screen.
type Model struct {
	view              ViewModel
	interpreters      []InterpreterOption
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
	detailsCache      map[string]InterpreterDetails
	highlighted       InterpreterDetails
	loadingPath       string
	loadingStarted    bool
}

// NewModel initialises a ready-to-use venvs Model.
func NewModel() Model {
	m := Model{
		view:            NewViewModel(),
		interpreters:    ListInterpreters(),
		startedWithVenv: os.Getenv("VIRTUAL_ENV") != "",
		detailsCache:    make(map[string]InterpreterDetails),
	}
	m.applySelection()

	// Initialize highlighted with the first interpreter's basic info
	if len(m.interpreters) > 0 {
		interpreter := m.interpreters[m.selected]
		m.highlighted = InterpreterDetails{
			Path: interpreter.Path,
			Kind: interpreter.Kind,
		}
		// Mark as loading so the view shows the loading state
		m.loadingPath = interpreter.Path
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	// Load highlighted interpreter details asynchronously
	if len(m.interpreters) > 0 {
		return m.queueDetailsLoad(m.interpreters[m.selected])
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.replModalOpen {
			return m.handleReplModalKey(msg)
		}
		return m.handleKey(msg)
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
	case detailsLoadedMsg:
		if msg.err == nil && msg.path == m.interpreters[m.selected].Path {
			m.highlighted = msg.details
			m.detailsCache[msg.path] = msg.details
			if m.packageSelected >= len(m.highlighted.Packages) {
				m.packageSelected = 0
			}
			m.packageScroll = 0
		}
		m.loadingPath = ""
	}

	// Trigger loading if we have a path to load but haven't started yet
	if !m.loadingStarted && m.loadingPath != "" && len(m.interpreters) > 0 {
		m.loadingStarted = true
		if _, exists := m.detailsCache[m.loadingPath]; !exists {
			for _, opt := range m.interpreters {
				if opt.Path == m.loadingPath {
					return m, m.queueDetailsLoad(opt)
				}
			}
		}
	}

	return m, nil
}

func (m *Model) handleReplModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.replModalOpen = false
		m.replStatus = ""
		return m, nil
	case tea.KeyEnter:
		m.replModalOpen = false
		m.replStatus = ""
		return m, m.launchSelectedREPL()
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.replModalOpen = false
		m.replStatus = ""
		return m, m.launchSelectedREPL()
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
		// Return to main menu
		return m, func() tea.Msg { return BackMsg{} }
	}
	if msg.Type == tea.KeyEnter {
		return m.handleEnter()
	}
	if m.focusPackages {
		return m.handlePackageNav(msg)
	}
	if m.dropdownOpen {
		return m.handleDropdownNav(msg)
	}
	return m, nil
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	if len(m.interpreters) == 0 {
		return m, nil
	}
	if m.focusPackages {
		m.focusPackages = false
		return m, nil
	}
	if !m.dropdownOpen {
		m.dropdownOpen = true
		cmd := m.queueDetailsLoad(m.interpreters[m.selected])
		return m, cmd
	}
	m.applySelection()
	m.dropdownOpen = false
	cmd := m.queueDetailsLoad(m.interpreters[m.selected])
	return m, cmd
}

func (m *Model) launchSelectedREPL() tea.Cmd {
	if len(m.interpreters) == 0 || m.selected >= len(m.interpreters) {
		return nil
	}
	replPath := m.interpreters[m.selected].Path
	if replPath == "" {
		return nil
	}
	replCmd := exec.Command(replPath)
	return tea.ExecProcess(replCmd, func(err error) tea.Msg {
		return replFinishedMsg{err: err}
	})
}

func (m *Model) handlePackageNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	return m, nil
}

func (m *Model) handleDropdownNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "k":
		if m.selected > 0 {
			m.selected--
			cmd := m.queueDetailsLoad(m.interpreters[m.selected])
			return m, cmd
		}
		return m, nil
	case "j":
		if m.selected < len(m.interpreters)-1 {
			m.selected++
			cmd := m.queueDetailsLoad(m.interpreters[m.selected])
			return m, cmd
		}
		return m, nil
	}
	switch msg.Type {
	case tea.KeyUp, tea.KeyCtrlP:
		if m.selected > 0 {
			m.selected--
			cmd := m.queueDetailsLoad(m.interpreters[m.selected])
			return m, cmd
		}
		return m, nil
	case tea.KeyDown, tea.KeyCtrlN:
		if m.selected < len(m.interpreters)-1 {
			m.selected++
			cmd := m.queueDetailsLoad(m.interpreters[m.selected])
			return m, cmd
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) applySelection() {
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
	if selected.Kind == InterpreterGlobal && m.startedWithVenv {
		m.activationCommand = fmt.Sprintf("deactivate # switched to global interpreter: %s", selected.Path)
		m.activationMessage = "Detected active venv at launch and selected global interpreter. Copied 'deactivate' command to clipboard."
	}
}

func (m *Model) refreshHighlightedDetails() {
	if len(m.interpreters) == 0 {
		m.highlighted = InterpreterDetails{}
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

func (m *Model) loadDetails(option InterpreterOption) InterpreterDetails {
	if option.Path == "" {
		return InterpreterDetails{}
	}
	if details, exists := m.detailsCache[option.Path]; exists {
		return details
	}
	details := option.Details()
	m.detailsCache[option.Path] = details
	return details
}

func (m *Model) queueDetailsLoad(option InterpreterOption) tea.Cmd {
	if option.Path == "" {
		return nil
	}

	// Check if already cached
	if details, exists := m.detailsCache[option.Path]; exists {
		m.highlighted = details
		return nil
	}

	// Mark as loading and return a command to load asynchronously
	m.loadingStarted = true
	m.loadingPath = option.Path
	m.highlighted = InterpreterDetails{
		Path: option.Path,
		Kind: option.Kind,
	}

	return func() tea.Msg {
		details := option.Details()
		return detailsLoadedMsg{
			path:    option.Path,
			details: details,
			err:     nil,
		}
	}
}

// SetSize sets the terminal dimensions on the model.
func (m *Model) SetSize(width, height int) {
	m.view.Width = width
	m.view.Height = height
}

// ActivationCommand returns the shell command to activate the selected interpreter.
func (m Model) ActivationCommand() string { return m.activationCommand }

// ActivationMessage returns a human-readable message about the activation command.
func (m Model) ActivationMessage() string { return m.activationMessage }

func (m *Model) scrollPackages(delta int) {
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
