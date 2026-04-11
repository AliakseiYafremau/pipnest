package main

import (
	"context"
	"pipnest/internal/requirements"
	"strings"

	"pipnest/internal/cheatsheet"
	pm "pipnest/internal/requirements/package_manager"
	"pipnest/internal/venvs"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type searchResult = requirements.Result

type searchDoneMsg = requirements.DoneMsg

type requirementsDoneMsg struct {
	Packages []pm.Dependency
	Err      error
}

type installPkgDoneMsg struct {
	Err error
}

type model struct {
	// Navigation
	currentScreen ScreenID
	menuCursor    int
	konamiIndex   int
	requirements  requirements.ViewModel

	// Packages screen
	input        textinput.Model
	width        int
	height       int
	query        string
	results      []searchResult
	selected     int
	listScroll   int
	detailScroll int
	focusedPane  int // 0 = list, 1 = detail
	loading      bool
	err          error

	// Requirements screen
	installedPackages []pm.Dependency
	selectedReqIdx    int
	reqLoading        bool
	reqErr            error
	reqInput          textinput.Model
	packageManager    pm.PackageManager
	reqMode           string // "list" o "install"

	// Cheatsheet screen
	cheatSearch       textinput.Model
	cheatSelected     int
	filteredCommands  []cheatsheet.CheatCommand
	cheatScrollOffset int
	cheatDetailScroll int
	cheatFocusedPane  int // 0 = list, 1 = detail

	// Venvs screen
	venvsApp *venvs.Model
}

var konamiSequence = []tea.KeyType{
	tea.KeyUp,
	tea.KeyUp,
	tea.KeyDown,
	tea.KeyDown,
	tea.KeyLeft,
	tea.KeyRight,
	tea.KeyLeft,
	tea.KeyRight,
}

func nextKonamiIndex(current int, key tea.KeyType) int {
	if current < len(konamiSequence) && key == konamiSequence[current] {
		return current + 1
	}
	if key == konamiSequence[0] {
		return 1
	}
	return 0
}

const (
	topInputHeight       = 5
	resultMouseStartLine = topInputHeight + 4
)

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Search PyPI packages..."

	cheatInput := textinput.New()
	cheatInput.Placeholder = "Search commands..."

	reqInput := textinput.New()
	reqInput.Placeholder = "Package name to install..."

	return model{
		currentScreen:     ScreenMainMenu,
		menuCursor:        0,
		requirements:      requirements.NewViewModel(),
		input:             ti,
		cheatSearch:       cheatInput,
		cheatSelected:     0,
		filteredCommands:  cheatsheet.CheatCommands,
		cheatScrollOffset: 0,
		reqInput:          reqInput,
		packageManager:    pm.NewPipManager(""),
		reqMode:           "list",
	}
}

func (m model) Init() tea.Cmd {
	return m.requirements.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle Ctrl+C globally
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	// Handle back navigation from venvs screen
	if _, ok := msg.(venvs.BackMsg); ok {
		m.currentScreen = ScreenMainMenu
		return m, nil
	}

	// Always propagate window size to all sub-models
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
		var cmd tea.Cmd
		m.requirements, cmd = m.requirements.Update(ws)
		if m.venvsApp != nil {
			updated, _ := m.venvsApp.Update(ws)
			if vm, ok := updated.(*venvs.Model); ok {
				m.venvsApp = vm
			}
		}
		_ = cmd
	}

	// Always forward requirements async messages regardless of current screen,
	// so background loads (pip list, metadata) complete even while on other screens.
	if m.currentScreen != ScreenRequirements {
		if requirements.IsAsyncMsg(msg) {
			var cmd tea.Cmd
			m.requirements, cmd = m.requirements.Update(msg)
			return m, cmd
		}
	}

	// Navegar según la pantalla actual
	switch m.currentScreen {
	case ScreenMainMenu:
		return m.updateMainMenu(msg)
	case ScreenPackages:
		return m.updatePackages(msg)
	case ScreenRequirements:
		return m.updateRequirements(msg)
	case ScreenVenvs:
		return m.updateVenvs(msg)
	case ScreenCheatSheet:
		return m.updateCheat(msg)
	case ScreenEasterEgg:
		return m.updateEasterEgg(msg)
	}

	return m, nil
}

// updateMainMenu: Lógica del menú principal
// Easter Egg (macarrones version): Arriba, Arriba, Abajo, Abajo, Izquierda, Derecha, Izquierda, Derecha
func (m model) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyUp:
			if m.menuCursor > 0 {
				m.menuCursor--
			}
			m.konamiIndex = nextKonamiIndex(m.konamiIndex, msg.Type)
		case tea.KeyDown:
			if m.menuCursor < len(MainMenuItems)-1 {
				m.menuCursor++
			}
			m.konamiIndex = nextKonamiIndex(m.konamiIndex, msg.Type)
		case tea.KeyLeft:
			m.konamiIndex = nextKonamiIndex(m.konamiIndex, msg.Type)
		case tea.KeyRight:
			m.konamiIndex = nextKonamiIndex(m.konamiIndex, msg.Type)
		case tea.KeyEnter:
			return m.activateMainMenuSelection()
		case tea.KeyRunes:
			runeKey := strings.ToLower(msg.String())
			if runeKey == "q" {
				return m, tea.Quit
			}
			if runeKey == "j" {
				if m.menuCursor < len(MainMenuItems)-1 {
					m.menuCursor++
				}
				m.konamiIndex = 0
				return m, nil
			}
			if runeKey == "k" {
				if m.menuCursor > 0 {
					m.menuCursor--
				}
				m.konamiIndex = 0
				return m, nil
			}
			if idx := findMainMenuIndexByInitial(runeKey); idx >= 0 {
				m.menuCursor = idx
				return m.activateMainMenuSelection()
			}
		default:
			m.konamiIndex = 0
		}
	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			if idx := mainMenuItemAtPosition(m, msg.X, msg.Y); idx >= 0 {
				m.menuCursor = idx
				return m.activateMainMenuSelection()
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	if m.konamiIndex >= len(konamiSequence) {
		m.currentScreen = ScreenEasterEgg
		m.konamiIndex = 0
	}

	return m, nil
}

func (m model) activateMainMenuSelection() (tea.Model, tea.Cmd) {
	selectedItem := MainMenuItems[m.menuCursor]
	m.currentScreen = selectedItem.Target
	m.menuCursor = 0
	m.konamiIndex = 0

	if m.currentScreen == ScreenRequirements {
		var sizeCmd tea.Cmd
		m.requirements, sizeCmd = m.requirements.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		if len(m.requirements.Packages) > 0 && !m.requirements.LoadingList {
			return m, sizeCmd
		}
		return m, tea.Batch(sizeCmd, m.requirements.Init())
	}

	if m.currentScreen == ScreenPackages {
		m.input.Focus()
	}
	if m.currentScreen == ScreenCheatSheet {
		m.cheatSearch.Focus()
		m.cheatSelected = 0
		m.cheatScrollOffset = 0
	}
	if m.currentScreen == ScreenVenvs {
		v := venvs.NewModel()
		v.SetSize(m.width, m.height)
		m.venvsApp = &v
		return m, m.venvsApp.Init()
	}

	return m, nil
}

func findMainMenuIndexByInitial(input string) int {
	if input == "" {
		return -1
	}
	for i, item := range MainMenuItems {
		label := strings.TrimSpace(strings.ToLower(item.Label))
		if label == "" {
			continue
		}
		if strings.HasPrefix(label, input) {
			return i
		}
	}
	return -1
}

func mainMenuItemAtPosition(m model, x, y int) int {
	if m.width < mainMenuMinWidth || m.height < mainMenuMinHeight {
		return -1
	}
	geom := computeMainMenuGeometry(m)
	if x < geom.startX || x >= geom.startX+geom.menuWidth {
		return -1
	}
	if y < geom.optionStartY || y >= geom.optionStartY+len(MainMenuItems) {
		return -1
	}
	idx := y - geom.optionStartY
	if idx < 0 || idx >= len(MainMenuItems) {
		return -1
	}
	return idx
}

// updatePackages: Lógica de búsqueda de paquetes
func (m model) updatePackages(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			m.currentScreen = ScreenMainMenu
			m.input.Blur()
			return m, nil
		}
		if len(m.results) > 0 {
			switch msg.Type {
			case tea.KeyLeft:
				m.focusedPane = 0
				return m, nil
			case tea.KeyRight:
				m.focusedPane = 1
				return m, nil
			case tea.KeyUp, tea.KeyCtrlP:
				if m.focusedPane == 0 {
					if m.selected > 0 {
						m.selected--
						m.detailScroll = 0
					}
				} else {
					m.detailScroll -= 3
					if m.detailScroll < 0 {
						m.detailScroll = 0
					}
				}
				return m, nil
			case tea.KeyDown, tea.KeyCtrlN:
				if m.focusedPane == 0 {
					if m.selected < len(m.results)-1 {
						m.selected++
						m.detailScroll = 0
					}
				} else {
					m.detailScroll += 3
				}
				return m, nil
			case tea.KeyCtrlU:
				m.detailScroll -= 5
				if m.detailScroll < 0 {
					m.detailScroll = 0
				}
				return m, nil
			case tea.KeyCtrlD:
				m.detailScroll += 5
				return m, nil
			}
		}
		if msg.Type == tea.KeyEnter {
			query := strings.TrimSpace(m.input.Value())
			if query == "" {
				m.query = ""
				m.results = nil
				m.err = nil
				m.loading = false
				m.detailScroll = 0
				return m, nil
			}

			m.query = query
			m.loading = true
			m.err = nil
			m.detailScroll = 0
			m.listScroll = 0
			m.focusedPane = 0
			return m, requirements.Search(query)
		}
	case tea.MouseMsg:
		if msg.Type == tea.MouseWheelUp {
			m.detailScroll--
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
			return m, nil
		}
		if msg.Type == tea.MouseWheelDown {
			m.detailScroll++
			return m, nil
		}
		if msg.Type == tea.MouseLeft && len(m.results) > 0 {
			index := msg.Y - resultMouseStartLine
			if index >= 0 && index < len(m.results)-m.listScroll {
				m.selected = index + m.listScroll
				m.detailScroll = 0
			}
			return m, nil
		}
	case searchDoneMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.results = nil
			m.selected = 0
			m.detailScroll = 0
			return m, nil
		}

		m.err = nil
		m.results = msg.Results
		m.selected = 0
		m.listScroll = 0
		m.detailScroll = 0
		m.focusedPane = 0
		cmds := make([]tea.Cmd, len(msg.Results))
		for i, r := range msg.Results {
			cmds[i] = requirements.FetchDescription(i, r.Name)
		}
		return m, tea.Batch(cmds...)

	case requirements.DescriptionLoadedMsg:
		if msg.Index >= 0 && msg.Index < len(m.results) {
			m.results[msg.Index] = msg.Result
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// updateRequirements: Lógica para pantalla de requirements
func (m model) updateRequirements(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyRunes {
		if !m.requirements.ModalOpen && !m.requirements.ActionModalOpen && !m.requirements.HelpModalOpen {
			runeKey := strings.ToLower(keyMsg.String())
			if runeKey == "q" {
				return m, tea.Quit
			}
		}
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEsc {
		if m.requirements.ModalOpen || m.requirements.ActionModalOpen || m.requirements.HelpModalOpen {
			var cmd tea.Cmd
			m.requirements, cmd = m.requirements.Update(msg)
			return m, cmd
		}

		m.currentScreen = ScreenMainMenu
		return m, nil
	}

	var cmd tea.Cmd
	m.requirements, cmd = m.requirements.Update(msg)
	return m, cmd
}

// updateVenvs: Lógica para pantalla de venvs
func (m model) updateVenvs(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.venvsApp == nil {
		return m, nil
	}
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
	}
	updated, cmd := m.venvsApp.Update(msg)
	if vm, ok := updated.(*venvs.Model); ok {
		m.venvsApp = vm
	}
	return m, cmd
}

// updateCheat: Lógica para pantalla de cheatsheet
func (m model) updateCheat(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			m.currentScreen = ScreenMainMenu
			m.cheatSearch.SetValue("")
			m.cheatSearch.Blur()
			m.cheatSelected = 0
			m.cheatScrollOffset = 0
			m.cheatDetailScroll = 0
			m.cheatFocusedPane = 0
			m.filteredCommands = cheatsheet.CheatCommands
			return m, nil
		}

		switch msg.Type {
		case tea.KeyLeft:
			m.cheatFocusedPane = 0
			return m, nil
		case tea.KeyRight:
			m.cheatFocusedPane = 1
			return m, nil
		case tea.KeyUp:
			if m.cheatFocusedPane == 1 {
				m.cheatDetailScroll -= 3
				if m.cheatDetailScroll < 0 {
					m.cheatDetailScroll = 0
				}
				return m, nil
			} else if m.cheatSelected > 0 {
				m.cheatSelected--
				m.cheatDetailScroll = 0
			}
		case tea.KeyDown:
			if m.cheatFocusedPane == 1 {
				m.cheatDetailScroll += 3
				return m, nil
			} else if m.cheatSelected < len(m.filteredCommands)-1 {
				m.cheatSelected++
				m.cheatDetailScroll = 0
			}
		case tea.KeyEnter:
			if m.cheatSelected >= 0 && m.cheatSelected < len(m.filteredCommands) {
				clipboard.WriteAll(m.filteredCommands[m.cheatSelected].Command)
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	var cmd tea.Cmd
	m.cheatSearch, cmd = m.cheatSearch.Update(msg)
	m.filteredCommands = cheatsheet.FilterCommands(cheatsheet.CheatCommands, m.cheatSearch.Value())

	if m.cheatSelected >= len(m.filteredCommands) {
		m.cheatSelected = len(m.filteredCommands) - 1
	}
	if m.cheatSelected < 0 {
		m.cheatSelected = 0
	}

	// keep in sync with renderCheatScreen: contentHeight = height-5, visibleLines = contentHeight-2
	visibleLines := m.height - 7
	if visibleLines < 1 {
		visibleLines = 1
	}
	if m.cheatSelected >= m.cheatScrollOffset+visibleLines {
		m.cheatScrollOffset = m.cheatSelected - visibleLines + 1
	}
	if m.cheatSelected < m.cheatScrollOffset {
		m.cheatScrollOffset = m.cheatSelected
	}
	if m.cheatScrollOffset > len(m.filteredCommands)-visibleLines {
		m.cheatScrollOffset = len(m.filteredCommands) - visibleLines
	}
	if m.cheatScrollOffset < 0 {
		m.cheatScrollOffset = 0
	}

	return m, cmd
}

func (m model) updateEasterEgg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			m.currentScreen = ScreenMainMenu
			m.menuCursor = 0
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m model) ActivationMessage() string {
	if m.venvsApp != nil {
		return m.venvsApp.ActivationMessage()
	}
	return ""
}

func (m model) View() string {
	switch m.currentScreen {
	case ScreenMainMenu:
		return renderMainMenu(m)
	case ScreenPackages:
		return renderPackagesScreen(m)
	case ScreenRequirements:
		return renderRequirementsScreen(m)
	case ScreenVenvs:
		return renderVenvsScreen(m)
	case ScreenCheatSheet:
		return renderCheatScreen(m)
	case ScreenEasterEgg:
		return renderEasterEgg(m)
	}
	return ""
}

// Funciones asincrónicas para requirements

func loadInstalledPackages(pkgMgr pm.PackageManager) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*1000*1000*1000) // 30s en nanosegundos
		defer cancel()

		packages, err := pkgMgr.List(ctx)
		return requirementsDoneMsg{Packages: packages, Err: err}
	}
}

func installPackage(pkgMgr pm.PackageManager, pkgName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*1000*1000*1000) // 60s
		defer cancel()

		err := pkgMgr.Install(ctx, pkgName)
		return installPkgDoneMsg{Err: err}
	}
}

func uninstallPackage(pkgMgr pm.PackageManager, pkgName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*1000*1000*1000) // 60s
		defer cancel()

		err := pkgMgr.Remove(ctx, pkgName)
		return installPkgDoneMsg{Err: err}
	}
}
