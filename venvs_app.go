package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

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
	}

	return m, nil
}

func (m venvsModel) handleReplModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.replModalOpen = false
		m.replStatus = ""
		return m, nil
	case tea.KeyEnter:
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

func (m venvsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m venvsModel) handleEnter() (tea.Model, tea.Cmd) {
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

func (m venvsModel) handlePackageNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m venvsModel) handleDropdownNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	return m, nil
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
func (m venvsModel) ActivationMessage() string  { return m.activationMessage }

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
