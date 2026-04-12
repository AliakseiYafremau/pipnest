//go:build linux || darwin
// +build linux darwin

package venvs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
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

type fileRunFinishedMsg struct {
	err error
}

type addInterpreterResultMsg struct {
	option InterpreterOption
	status string
	err    error
}

type addInterpreterFormData struct {
	modeNew      bool
	existingPath string
	newPath      string
	basePython   string
	inheritSite  bool
}

// Model holds the full state of the venvs screen.
type Model struct {
	view               ViewModel
	interpreters       []InterpreterOption
	selected           int
	dropdownOpen       bool
	focusPackages      bool
	packageSelected    int
	packageScroll      int
	keybindsModalOpen  bool
	runFileModalOpen   bool
	runFilePath        string
	runFileStatus      string
	replModalOpen      bool
	replStatus         string
	addModalOpen       bool
	addForm            *huh.Form
	addStatus          string
	addLoading         bool
	addSpinner         spinner.Model
	addFormData        *addInterpreterFormData
	addCreating        bool
	globalInterpreters []InterpreterOption
	activationCommand  string
	activationMessage  string
	startedWithVenv    bool
	detailsCache       map[string]InterpreterDetails
	highlighted        InterpreterDetails
	loadingPath        string
	loadingStarted     bool
}

// NewModel initialises a ready-to-use venvs Model.
func NewModel() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		FPS:    time.Second / 10,
	}
	m := Model{
		view:               NewViewModel(),
		interpreters:       ListInterpreters(),
		dropdownOpen:       true,
		startedWithVenv:    os.Getenv("VIRTUAL_ENV") != "",
		detailsCache:       make(map[string]InterpreterDetails),
		globalInterpreters: globalInterpretersFromPath(),
		addSpinner:         sp,
	}
	m.restoreSelectionFromProject()
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

func (m *Model) IsLoading() bool {
	return m.addLoading || m.loadingPath != ""
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case addInterpreterResultMsg:
		m.addCreating = false
		if msg.err != nil {
			m.addStatus = msg.err.Error()
			m.addModalOpen = true
			m.addForm = m.buildAddForm()
			return m, m.addForm.Init()
		}
		m.addStatus = msg.status
		m.addModalOpen = false
		m.addForm = nil
		m.addFormData = nil
		if msg.option.Path != "" {
			m.reloadInterpreters(msg.option.Path)
			m.applySelection()
			if len(m.interpreters) > 0 {
				return m, m.queueDetailsLoad(m.interpreters[m.selected])
			}
		}
		return m, nil
	case tea.KeyMsg:
		if m.runFileModalOpen {
			return m.handleRunFileModalKey(msg)
		}
		if m.keybindsModalOpen {
			return m.handleKeybindsModalKey(msg)
		}
		if m.addModalOpen {
			return m.handleAddModalMessage(msg)
		}
		if m.replModalOpen {
			return m.handleReplModalKey(msg)
		}
		return m.handleKey(msg)
	case tea.MouseMsg:
		if m.keybindsModalOpen {
			if msg.Type == tea.MouseLeft {
				m.keybindsModalOpen = false
			}
			return m, nil
		}
		if m.runFileModalOpen {
			return m, nil
		}
		switch msg.Type {
		case tea.MouseLeft:
			return m.handleMouseClick(msg)
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
	case fileRunFinishedMsg:
		if msg.err != nil {
			m.runFileStatus = "Execution failed: " + msg.err.Error()
		} else {
			m.runFileStatus = "Execution finished"
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.addSpinner, cmd = m.addSpinner.Update(msg)
		return m, cmd
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

func (m *Model) handleKeybindsModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc || msg.String() == "?" || msg.String() == "q" {
		m.keybindsModalOpen = false
		return m, nil
	}
	return m, nil
}

func (m *Model) handleAddModalMessage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "q" || msg.Type == tea.KeyEsc {
		m.addModalOpen = false
		m.addForm = nil
		m.addFormData = nil
		m.addStatus = ""
		return m, nil
	}
	if m.addForm != nil && m.addFormData != nil {
		if focused := m.addForm.GetFocusedField(); focused != nil {
			key := focused.GetKey()
			if key == "mode" {
				if msg.Type == tea.KeyLeft {
					m.addFormData.modeNew = !m.addFormData.modeNew
					m.addForm = m.buildAddForm()
					return m, m.addForm.Init()
				}
				if msg.Type == tea.KeyRight {
					m.addFormData.modeNew = !m.addFormData.modeNew
					m.addForm = m.buildAddForm()
					return m, m.addForm.Init()
				}
				if msg.Type == tea.KeyEnter {
					return m, m.addForm.NextGroup()
				}
			}

			if msg.Type == tea.KeyEnter {
				switch key {
				case "existing_path":
					return m.submitAddInterpreterForm()
				case "new_root", "base_python":
					return m, m.addForm.NextField()
				case "inherit_site":
					return m.submitAddInterpreterForm()
				}
			}
		}
	}
	formModel, cmd := m.addForm.Update(msg)
	if f, ok := formModel.(*huh.Form); ok {
		m.addForm = f
	}
	if m.addForm.State == huh.StateCompleted {
		return m.submitAddInterpreterForm()
	}
	return m, cmd
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
	if msg.String() == "?" {
		m.keybindsModalOpen = true
		return m, nil
	}
	if msg.String() == "x" {
		m.runFileModalOpen = true
		if m.runFilePath == "" {
			m.runFilePath = ""
		}
		return m, nil
	}
	if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
		m.applySelection()
		return m, tea.Quit
	}
	if msg.Type == tea.KeyRight {
		if len(m.highlighted.Packages) > 0 {
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
	if msg.String() == "a" {
		m.addModalOpen = true
		m.replModalOpen = false
		m.addStatus = ""
		m.addLoading = false
		newPath := ".venv"
		if wd, err := os.Getwd(); err == nil {
			newPath = filepath.Join(wd, ".venv")
		}
		m.globalInterpreters = preferredBaseInterpreters()
		basePython := ""
		if len(m.globalInterpreters) > 0 {
			basePython = m.globalInterpreters[0].Path
		}
		m.addFormData = &addInterpreterFormData{
			modeNew:      true,
			existingPath: "",
			newPath:      newPath,
			basePython:   basePython,
			inheritSite:  false,
		}
		m.addForm = m.buildAddForm()
		return m, m.addForm.Init()
	}
	if msg.Type == tea.KeyEsc {
		m.applySelection()
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

func preferredBaseInterpreters() []InterpreterOption {
	all := globalInterpretersFromPath()
	all = dedupeGlobalInterpreterChoices(all)
	preferred := make([]InterpreterOption, 0, len(all))
	for _, option := range all {
		if isVenvLikeInterpreterPath(option.Path) {
			continue
		}
		preferred = append(preferred, option)
	}
	if len(preferred) > 0 {
		return preferred
	}
	return all
}

func dedupeGlobalInterpreterChoices(options []InterpreterOption) []InterpreterOption {
	if len(options) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(options)*2)
	deduped := make([]InterpreterOption, 0, len(options))
	for _, option := range options {
		if option.Path == "" {
			continue
		}
		if _, exists := seen[option.Path]; exists {
			continue
		}
		if canonical := canonicalPath(option.Path); canonical != "" {
			dedupeKey := canonical + "\x00" + filepath.Base(option.Path)
			if _, exists := seen[dedupeKey]; exists {
				continue
			}
			seen[dedupeKey] = struct{}{}
		}
		seen[option.Path] = struct{}{}
		deduped = append(deduped, option)
	}

	return deduped
}

func (m *Model) buildAddForm() *huh.Form {
	if m.addFormData == nil {
		m.addFormData = &addInterpreterFormData{modeNew: true}
	}
	data := m.addFormData

	pythonOptions := make([]huh.Option[string], 0, len(m.globalInterpreters))
	for _, option := range m.globalInterpreters {
		label := option.Path
		if option.Label != "" {
			label = fmt.Sprintf("%s (%s)", option.Label, option.Path)
		}
		pythonOptions = append(pythonOptions, huh.NewOption(label, option.Path))
	}
	if len(pythonOptions) == 0 {
		pythonOptions = append(pythonOptions, huh.NewOption("No global Python found", ""))
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("mode").
				Title("Mode (left/right): create new environment?").
				Affirmative("new").
				Negative("existing").
				Value(&data.modeNew),
		),
		huh.NewGroup(
			huh.NewInput().
				Key("existing_path").
				Title("Existing interpreter path").
				Placeholder("/path/to/python").
				Value(&data.existingPath),
		).WithHideFunc(func() bool {
			return data.modeNew
		}),
		huh.NewGroup(
			huh.NewInput().
				Key("new_root").
				Title("Path for new environment root").
				Placeholder("/path/to/project/.venv").
				Value(&data.newPath),
			huh.NewSelect[string]().
				Key("base_python").
				Title("Base global Python").
				Options(pythonOptions...).
				Value(&data.basePython),
			huh.NewConfirm().
				Key("inherit_site").
				Title("Inherit global site packages?").
				Affirmative("yes").
				Negative("no").
				Value(&data.inheritSite),
		).WithHideFunc(func() bool {
			return !data.modeNew
		}),
	).WithShowHelp(false)
}

func (m *Model) submitAddInterpreterForm() (tea.Model, tea.Cmd) {
	if m.addFormData == nil {
		m.addStatus = "form state unavailable"
		m.addForm = m.buildAddForm()
		return m, m.addForm.Init()
	}

	if m.addFormData.modeNew {
		targetRoot := strings.TrimSpace(m.addFormData.newPath)
		if targetRoot == "" {
			m.addStatus = "Provide a target path for the new environment"
			m.addForm = m.buildAddForm()
			return m, m.addForm.Init()
		}
		if strings.TrimSpace(m.addFormData.basePython) == "" {
			m.addStatus = "Select a global Python to create the environment"
			m.addForm = m.buildAddForm()
			return m, m.addForm.Init()
		}
		m.addCreating = true
		m.addForm = nil
		return m, tea.Batch(createNewInterpreterCmd(m.addFormData.basePython, targetRoot, m.addFormData.inheritSite), m.addSpinner.Tick)
	}

	existingPath := strings.TrimSpace(m.addFormData.existingPath)
	if existingPath == "" {
		m.addStatus = "Provide a path to an existing Python interpreter"
		m.addForm = m.buildAddForm()
		return m, m.addForm.Init()
	}
	m.addCreating = true
	m.addForm = nil
	return m, tea.Batch(useExistingInterpreterCmd(existingPath), m.addSpinner.Tick)
}

func (m *Model) reloadInterpreters(selectedPath string) {
	m.interpreters = ListInterpreters()
	if len(m.interpreters) == 0 {
		m.selected = 0
		return
	}
	for i, option := range m.interpreters {
		if option.Path == selectedPath {
			m.selected = i
			return
		}
	}
	m.selected = 0
}

func createNewInterpreterCmd(basePython, targetRoot string, inheritSite bool) tea.Cmd {
	return func() tea.Msg {
		if absTarget, err := filepath.Abs(targetRoot); err == nil {
			if sameOrWithinPath(canonicalPath(basePython), absTarget) {
				return addInterpreterResultMsg{err: fmt.Errorf("base python cannot be inside the target environment path")}
			}
		}

		args := []string{"-m", "venv"}
		if inheritSite {
			args = append(args, "--system-site-packages")
		}
		args = append(args, targetRoot)

		cmd := exec.Command(basePython, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			message := strings.TrimSpace(string(output))
			if message == "" {
				message = err.Error()
			}

			if option, optionErr := interpreterOptionFromRootPath(targetRoot); optionErr == nil {
				if rememberErr := rememberInterpreter(option); rememberErr != nil {
					return addInterpreterResultMsg{err: rememberErr}
				}
				status := "Environment created and selected"
				if strings.Contains(strings.ToLower(message), "ensurepip") {
					status = "Environment created, but pip bootstrap failed (ensurepip). Install your system's venv package if needed."
				} else {
					status = "Environment created with warnings: " + firstLine(message)
				}
				return addInterpreterResultMsg{option: option, status: status}
			}

			return addInterpreterResultMsg{err: fmt.Errorf("create environment failed: %s", message)}
		}

		option, err := interpreterOptionFromRootPath(targetRoot)
		if err != nil {
			return addInterpreterResultMsg{err: err}
		}
		if err := rememberInterpreter(option); err != nil {
			return addInterpreterResultMsg{err: err}
		}
		return addInterpreterResultMsg{option: option, status: "Environment created and selected"}
	}
}

func firstLine(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if idx := strings.IndexByte(trimmed, '\n'); idx >= 0 {
		return strings.TrimSpace(trimmed[:idx])
	}
	return trimmed
}

func useExistingInterpreterCmd(path string) tea.Cmd {
	return func() tea.Msg {
		option, err := interpreterOptionFromPath(path)
		if err != nil {
			return addInterpreterResultMsg{err: err}
		}
		if err := rememberInterpreter(option); err != nil {
			return addInterpreterResultMsg{err: err}
		}
		return addInterpreterResultMsg{option: option, status: "Interpreter selected"}
	}
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

func (m *Model) launchSelectedFile(path string) tea.Cmd {
	if len(m.interpreters) == 0 || m.selected >= len(m.interpreters) {
		return nil
	}
	interpreter := m.interpreters[m.selected].Path
	if interpreter == "" || strings.TrimSpace(path) == "" {
		return nil
	}
	// Keep the terminal open after execution so the user can read output,
	// then return to the TUI after pressing Enter.
	runCmd := exec.Command(
		"sh",
		"-c",
		`"$1" "$2"; status=$?; printf "\nPress Enter to return to Pipnest..."; IFS= read -r _; exit $status`,
		"pipnest",
		interpreter,
		path,
	)
	return tea.ExecProcess(runCmd, func(err error) tea.Msg {
		return fileRunFinishedMsg{err: err}
	})
}

func (m *Model) handleRunFileModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.runFileModalOpen = false
		return m, nil
	case tea.KeyEnter:
		path := strings.TrimSpace(m.runFilePath)
		if path == "" {
			m.runFileStatus = "Provide a file path to execute"
			return m, nil
		}
		m.runFileModalOpen = false
		m.runFileStatus = ""
		return m, m.launchSelectedFile(path)
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.runFilePath) > 0 {
			m.runFilePath = m.runFilePath[:len(m.runFilePath)-1]
		}
		return m, nil
	}
	if len(msg.Runes) > 0 {
		m.runFilePath += string(msg.Runes)
	}
	return m, nil
}

func (m *Model) handleMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	bodyHeight := m.view.Height - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	x, y := msg.X, msg.Y
	if y == m.view.Height-1 {
		switch m.legendActionAtX(x) {
		case "enter":
			return m.handleEnter()
		case "repl":
			return m, m.launchSelectedREPL()
		case "run-file":
			m.runFileModalOpen = true
			return m, nil
		case "help":
			m.keybindsModalOpen = true
			return m, nil
		case "menu":
			if m.focusPackages {
				m.focusPackages = false
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }
		case "quit":
			m.applySelection()
			return m, tea.Quit
		}
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
	rowWidth := leftWidth + rightWidth
	rowX := 0
	if m.view.Width > rowWidth {
		rowX = (m.view.Width - rowWidth) / 2
	}
	if y < 0 || y >= panelHeight {
		return m, nil
	}

	leftXStart := rowX
	leftXEnd := rowX + leftWidth
	rightXStart := leftXEnd
	rightXEnd := rightXStart + rightWidth

	if x >= leftXStart && x < leftXEnd {
		if len(m.interpreters) == 0 {
			return m, nil
		}
		m.focusPackages = false
		innerHeight := max(1, panelHeight-4)
		availableRows := innerHeight - 2 // title + blank
		if availableRows < 0 {
			availableRows = 0
		}
		start := 0
		end := len(m.interpreters)
		if availableRows < len(m.interpreters) {
			start = clamp(m.selected-(availableRows/2), 0, max(0, len(m.interpreters)-availableRows))
			end = start + availableRows
		}
		rowStartY := 4 // border(1) + padding(1) + title(1) + blank(1)
		idx := start + (y - rowStartY)
		if y >= rowStartY && idx >= start && idx < end {
			m.selected = idx
			m.applySelection()
			return m, m.queueDetailsLoad(m.interpreters[m.selected])
		}
		return m, nil
	}

	if x >= rightXStart && x < rightXEnd {
		m.focusPackages = true
		details := m.highlighted
		if len(details.Packages) == 0 {
			return m, nil
		}
		innerHeight := max(1, panelHeight-4)
		availableRows := packageVisibleLines(innerHeight, detailsHeaderLines())
		if availableRows < 1 {
			return m, nil
		}
		rowStartY := 12 // border+padding+details header lines
		idx := m.packageScroll + (y - rowStartY)
		if y >= rowStartY && idx >= m.packageScroll && idx < min(len(details.Packages), m.packageScroll+availableRows) {
			m.packageSelected = idx
		}
	}

	return m, nil
}

func (m *Model) legendActionAtX(x int) string {
	if x < 0 {
		return ""
	}
	segments := []struct {
		label  string
		action string
	}{
		{label: "Enter: select", action: "enter"},
		{label: "a: add", action: ""},
		{label: "r: REPL", action: "repl"},
		{label: "x: run file", action: "run-file"},
		{label: "Esc: menu", action: "menu"},
		{label: "q: quit", action: "quit"},
		{label: "?: help", action: "help"},
	}
	sep := "  |  "
	cursor := 0
	for i, seg := range segments {
		start := cursor
		end := start + len([]rune(seg.label))
		if x >= start && x < end {
			return seg.action
		}
		cursor = end
		if i < len(segments)-1 {
			cursor += len([]rune(sep))
		}
	}
	return ""
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
	m.persistSelectionForProject(selected)

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

	loadCmd := func() tea.Msg {
		details := option.Details()
		return detailsLoadedMsg{
			path:    option.Path,
			details: details,
			err:     nil,
		}
	}

	return tea.Batch(loadCmd, m.addSpinner.Tick)
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
