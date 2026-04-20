package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rotlerxd/pipnest/internal/backends"
	"github.com/Rotlerxd/pipnest/internal/service"
	"github.com/Rotlerxd/pipnest/internal/ui/components"
	"github.com/Rotlerxd/pipnest/internal/venv"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type packageListLoadedMsg struct {
	packages []backends.Package
	err      error
}

type venvListLoadedMsg struct {
	venvs []venv.Venv
	err   error
}

type appService interface {
	ListPackages(ctx context.Context) ([]backends.Package, error)
	ListVenv(ctx context.Context) ([]venv.Venv, error)
}

type AppModel struct {
	exitKeyMap components.ExitKeyMap
	service    appService

	width  int
	height int

	searchQuery string

	packages []backends.Package
	filtered []backends.Package
	selected int

	venvs  []venv.Venv
	status string
}

func NewAppModel(exitKeyMap components.ExitKeyMap, service *service.Service) *AppModel {
	return &AppModel{
		exitKeyMap:  exitKeyMap,
		service:     service,
		searchQuery: "",
		status:      "loading...",
	}
}

func (m *AppModel) Init() tea.Cmd {
	return tea.Batch(loadPackagesCmd(m.service), loadVenvsCmd(m.service))
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case packageListLoadedMsg:
		if msg.err != nil {
			m.status = "packages: " + msg.err.Error()
			return m, nil
		}
		m.packages = msg.packages
		m.applyPackageFilter()
		m.status = fmt.Sprintf("packages: %d", len(m.packages))
		return m, nil
	case venvListLoadedMsg:
		if msg.err != nil {
			m.status = "venv: " + msg.err.Error()
			return m, nil
		}
		m.venvs = msg.venvs
		m.status = fmt.Sprintf("packages: %d, venv: %d", len(m.packages), len(m.venvs))
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, m.exitKeyMap.Exit) {
			return m, tea.Quit
		}

		switch msg.String() {
		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.applyPackageFilter()
			}
			return m, nil
		case "ctrl+r":
			m.status = "refreshing..."
			return m, tea.Batch(loadPackagesCmd(m.service), loadVenvsCmd(m.service))
		case "up":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down":
			if m.selected < len(m.filtered)-1 {
				m.selected++
			}
			return m, nil
		}

		if len(msg.Runes) > 0 && msg.Alt == false && msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
			m.applyPackageFilter()
			return m, nil
		}
	}

	return m, nil
}

func (m *AppModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "loading..."
	}

	frameWidth := m.width
	frameHeight := m.height
	contentWidth := frameWidth - 2
	contentHeight := frameHeight - 2
	if contentWidth < 20 || contentHeight < 10 {
		return "Terminal too small"
	}

	gap := components.UI.Gap
	leftWidth := (contentWidth - gap) / 2
	rightWidth := contentWidth - leftWidth - gap

	layoutHeight := contentHeight - 1
	if layoutHeight < 3 {
		layoutHeight = 3
	}

	verticalAvailable := layoutHeight - gap
	if verticalAvailable < 2 {
		verticalAvailable = 2
	}
	leftTopHeight := int(float64(verticalAvailable) * components.UI.LeftTopRatio)
	if leftTopHeight < 1 {
		leftTopHeight = 1
	}
	if leftTopHeight > verticalAvailable-1 {
		leftTopHeight = verticalAvailable - 1
	}
	leftBottomHeight := verticalAvailable - leftTopHeight

	rightBottomHeight := components.UI.RightBottomHeight
	if rightBottomHeight < 1 {
		rightBottomHeight = 1
	}
	if rightBottomHeight > verticalAvailable-1 {
		rightBottomHeight = verticalAvailable - 1
	}
	rightTopHeight := verticalAvailable - rightBottomHeight

	searchView := components.RenderSearchBox(m.searchQuery, leftWidth-6)
	leftTopBody := searchView + "\n\n" +
		components.RenderPackagesList(m.filtered, m.selected, leftTopHeight-5)
	leftBottomBody := components.RenderVenvList(m.venvs, leftBottomHeight-2)

	rightTopBody := lipgloss.NewStyle().Foreground(components.UI.MutedTextColor).Render("Description placeholder")
	rightBottomBody := lipgloss.NewStyle().Foreground(components.UI.MutedTextColor).Render("Console log placeholder")

	leftTop := components.RenderPanel("Installed packages", leftTopBody, leftWidth, leftTopHeight)
	leftBottom := components.RenderPanel(".venv", leftBottomBody, leftWidth, leftBottomHeight)
	rightTop := components.RenderPanel("Description", rightTopBody, rightWidth, rightTopHeight)
	rightBottom := components.RenderPanel("Console log", rightBottomBody, rightWidth, rightBottomHeight)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, leftTop, "", leftBottom)
	rightColumn := lipgloss.JoinVertical(lipgloss.Left, rightTop, "", rightBottom)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, strings.Repeat(" ", gap), rightColumn)

	statusLine := lipgloss.NewStyle().Foreground(components.UI.MutedTextColor).Render(m.status)
	rootContent := lipgloss.JoinVertical(lipgloss.Left, layout, statusLine)
	return components.RenderRootFrame(rootContent, frameWidth, frameHeight)
}

func loadPackagesCmd(s appService) tea.Cmd {
	return func() tea.Msg {
		if s == nil {
			return packageListLoadedMsg{packages: nil, err: nil}
		}
		packages, err := s.ListPackages(context.Background())
		return packageListLoadedMsg{packages: packages, err: err}
	}
}

func loadVenvsCmd(s appService) tea.Cmd {
	return func() tea.Msg {
		if s == nil {
			return venvListLoadedMsg{venvs: nil, err: nil}
		}
		venvs, err := s.ListVenv(context.Background())
		return venvListLoadedMsg{venvs: venvs, err: err}
	}
}

func (m *AppModel) applyPackageFilter() {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" {
		m.filtered = append([]backends.Package(nil), m.packages...)
	} else {
		filtered := make([]backends.Package, 0, len(m.packages))
		for _, p := range m.packages {
			if strings.Contains(strings.ToLower(p.Name), query) {
				filtered = append(filtered, p)
			}
		}
		m.filtered = filtered
	}

	if m.selected >= len(m.filtered) {
		m.selected = len(m.filtered) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}
