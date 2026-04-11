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
	input    textinput.Model
	width    int
	height   int
	query    string
	results  []searchResult
	selected int
	loading  bool
	err      error

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
	resultMouseStartLine = topInputHeight + 5
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
			selectedItem := MainMenuItems[m.menuCursor]
			m.currentScreen = selectedItem.Target
			m.menuCursor = 0
			m.konamiIndex = 0

			if m.currentScreen == ScreenRequirements {
				var cmd tea.Cmd
				m.requirements, cmd = m.requirements.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
				return m, cmd
			}

			if m.currentScreen == ScreenPackages {
				m.input.Focus()
			}
			if m.currentScreen == ScreenRequirements {
				// Cargar lista de paquetes
				m.reqLoading = true
				m.reqErr = nil
				return m, loadInstalledPackages(m.packageManager)
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
		case tea.KeyRunes:
			if msg.String() == "q" {
				return m, tea.Quit
			}
		default:
			m.konamiIndex = 0
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
			case tea.KeyUp, tea.KeyCtrlP:
				if m.selected > 0 {
					m.selected--
				}
				return m, nil
			case tea.KeyDown, tea.KeyCtrlN:
				if m.selected < len(m.results)-1 {
					m.selected++
				}
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
				return m, nil
			}

			m.query = query
			m.loading = true
			m.err = nil
			return m, requirements.Search(query)
		}
	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft && len(m.results) > 0 {
			index := msg.Y - resultMouseStartLine
			if index >= 0 && index < len(m.results) {
				m.selected = index
			}
			return m, nil
		}
	case searchDoneMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.results = nil
			m.selected = 0
			return m, nil
		}

		m.err = nil
		m.results = msg.Results
		m.selected = 0
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
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEsc {
		if m.requirements.ModalOpen {
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
			m.filteredCommands = cheatsheet.CheatCommands
			return m, nil
		}

		// Solo procesar navegación si El Input NO tiene focus
		if !m.cheatSearch.Focused() {
			// Copiar comando al portapapeles con Enter
			if msg.Type == tea.KeyEnter {
				if m.cheatSelected >= 0 && m.cheatSelected < len(m.filteredCommands) {
					cmd := m.filteredCommands[m.cheatSelected]
					clipboard.WriteAll(cmd.Command)
				}
				return m, nil
			}

			// Navegación con Tab para cambiar entre input y lista
			if msg.Type == tea.KeyTab {
				m.cheatSearch.Focus()
				return m, nil
			}

			// Navegación en la lista
			switch msg.Type {
			case tea.KeyUp:
				if m.cheatSelected > 0 {
					m.cheatSelected--
				}
			case tea.KeyDown:
				if m.cheatSelected < len(m.filteredCommands)-1 {
					m.cheatSelected++
				}
			case tea.KeyPgUp:
				// Mismo cálculo que renderCheatScreen: contentHeight = m.height-11, visibleLines = contentHeight-4
				visibleLines := m.height - 15
				if visibleLines < 1 {
					visibleLines = 1
				}
				m.cheatSelected -= visibleLines
				if m.cheatSelected < 0 {
					m.cheatSelected = 0
				}
			case tea.KeyPgDown:
				visibleLines := m.height - 15
				if visibleLines < 1 {
					visibleLines = 1
				}
				m.cheatSelected += visibleLines
				if m.cheatSelected >= len(m.filteredCommands) {
					m.cheatSelected = len(m.filteredCommands) - 1
				}
			case tea.KeyHome:
				m.cheatSelected = 0
			case tea.KeyEnd:
				m.cheatSelected = len(m.filteredCommands) - 1
			}
		} else {
			// Cuando el input tiene focus, permitir desenfocarse con la tecla de escape
			if msg.Type == tea.KeyTab {
				m.cheatSearch.Blur()
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	// Actualizar input de búsqueda
	var cmd tea.Cmd
	m.cheatSearch, cmd = m.cheatSearch.Update(msg)

	// Filtrar comandos según búsqueda
	m.filteredCommands = cheatsheet.FilterCommands(cheatsheet.CheatCommands, m.cheatSearch.Value())

	// Ajustar la selección si está fuera de rango después del filtrado
	if m.cheatSelected >= len(m.filteredCommands) {
		m.cheatSelected = len(m.filteredCommands) - 1
	}
	if m.cheatSelected < 0 {
		m.cheatSelected = 0
	}

	// Recalcular scroll offset para mantenerlo dentro de los límites
	// Mismo cálculo que renderCheatScreen: contentHeight = m.height-11, visibleLines = contentHeight-4
	visibleLines := m.height - 15
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
