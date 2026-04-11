package main

import (
	"strings"

	"pipnest/internal/cheatsheet"
	"pipnest/internal/requirements"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type searchResult = requirements.Result

type searchDoneMsg = requirements.DoneMsg

type model struct {
	// Navigation
	currentScreen ScreenID
	menuCursor    int

	// Packages screen
	input    textinput.Model
	width    int
	height   int
	query    string
	results  []searchResult
	selected int
	loading  bool
	err      error

	// Cheatsheet screen
	cheatSearch       textinput.Model
	cheatSelected     int
	filteredCommands  []cheatsheet.CheatCommand
	cheatScrollOffset int
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

	return model{
		currentScreen:     ScreenMainMenu,
		menuCursor:        0,
		input:             ti,
		cheatSearch:       cheatInput,
		cheatSelected:     0,
		filteredCommands:  cheatsheet.CheatCommands,
		cheatScrollOffset: 0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle Ctrl+C globally
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyCtrlC {
		return m, tea.Quit
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
	}

	return m, nil
}

// updateMainMenu: Lógica del menú principal
func (m model) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case tea.KeyDown:
			if m.menuCursor < len(MainMenuItems)-1 {
				m.menuCursor++
			}
		case tea.KeyEnter:
			selectedItem := MainMenuItems[m.menuCursor]
			m.currentScreen = selectedItem.Target
			m.menuCursor = 0
			if m.currentScreen == ScreenPackages {
				m.input.Focus()
			}
			if m.currentScreen == ScreenCheatSheet {
				m.cheatSearch.Focus()
				m.cheatSelected = 0
				m.cheatScrollOffset = 0
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			m.currentScreen = ScreenMainMenu
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// updateVenvs: Lógica para pantalla de venvs
func (m model) updateVenvs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			m.currentScreen = ScreenMainMenu
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
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
				visibleLines := (m.height - 8) / 3 // Estimar líneas visibles
				m.cheatSelected -= visibleLines
				if m.cheatSelected < 0 {
					m.cheatSelected = 0
				}
			case tea.KeyPgDown:
				visibleLines := (m.height - 8) / 3
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
	visibleLines := (m.height - 8) / 3
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
	}
	return ""
}
