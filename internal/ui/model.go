package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rotlerxd/pipnest/internal/backends"
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

type packageDetailsLoadedMsg struct {
	name    string
	details backends.PackageDetails
	err     error
}

type appService interface {
	ListPackages(ctx context.Context) ([]backends.Package, error)
	ListVenv(ctx context.Context) ([]venv.Venv, error)
	ShowPackage(ctx context.Context, packageName string) (backends.PackageDetails, error)
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

	packageDetails backends.PackageDetails
	detailsLoading bool
	detailsError   string
}

func NewAppModel(exitKeyMap components.ExitKeyMap, service appService) *AppModel {
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
			m.packageDetails = backends.PackageDetails{}
			m.detailsLoading = false
			m.detailsError = ""
			return m, nil
		}
		m.packages = msg.packages
		m.applyPackageFilter()
		m.status = fmt.Sprintf("packages: %d", len(m.packages))
		return m, m.loadSelectedPackageDetailsCmd()
	case venvListLoadedMsg:
		if msg.err != nil {
			m.status = "venv: " + msg.err.Error()
			return m, nil
		}
		m.venvs = msg.venvs
		m.status = fmt.Sprintf("packages: %d, venv: %d", len(m.packages), len(m.venvs))
		return m, nil
	case packageDetailsLoadedMsg:
		if msg.name != m.selectedPackageName() {
			return m, nil
		}

		m.detailsLoading = false
		if msg.err != nil {
			m.packageDetails = backends.PackageDetails{}
			m.detailsError = msg.err.Error()
			return m, nil
		}

		m.packageDetails = msg.details
		m.detailsError = ""
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, m.exitKeyMap.Exit) {
			return m, tea.Quit
		}

		prevSelected := m.selectedPackageName()

		switch msg.String() {
		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.applyPackageFilter()
			}
			return m, m.maybeReloadPackageDetailsCmd(prevSelected)
		case "ctrl+r":
			m.status = "refreshing..."
			m.detailsLoading = false
			m.detailsError = ""
			return m, tea.Batch(loadPackagesCmd(m.service), loadVenvsCmd(m.service))
		case "up":
			if m.selected > 0 {
				m.selected--
			}
			return m, m.maybeReloadPackageDetailsCmd(prevSelected)
		case "down":
			if m.selected < len(m.filtered)-1 {
				m.selected++
			}
			return m, m.maybeReloadPackageDetailsCmd(prevSelected)
		}

		if len(msg.Runes) > 0 && msg.Alt == false && msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
			m.applyPackageFilter()
			return m, m.maybeReloadPackageDetailsCmd(prevSelected)
		}
	}

	return m, nil
}

func (m *AppModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "loading..."
	}

	gap := components.UI.Gap
	leftWidth := (m.width - gap) / 2
	rightWidth := m.width - leftWidth - gap

	contentHeight := m.height - 1
	if contentHeight < 10 {
		contentHeight = m.height
	}
	leftTopHeight := int(float64(contentHeight) * components.UI.LeftTopRatio)
	if leftTopHeight < 8 {
		leftTopHeight = 8
	}
	leftBottomHeight := contentHeight - leftTopHeight - gap
	if leftBottomHeight < 4 {
		leftBottomHeight = 4
	}
	rightBottomHeight := components.UI.RightBottomHeight
	if rightBottomHeight > contentHeight-4 {
		rightBottomHeight = 4
	}
	rightTopHeight := contentHeight - rightBottomHeight - gap

	searchView := components.RenderSearchBox(m.searchQuery, leftWidth-6)
	leftTopBody := searchView + "\n\n" +
		components.RenderPackagesList(m.filtered, m.selected, leftTopHeight-5)
	leftBottomBody := components.RenderVenvList(m.venvs, leftBottomHeight-2)

	rightTopBody := components.RenderPackageDetails(m.selectedPackageName(), m.packageDetails, m.detailsLoading, m.detailsError)
	rightBottomBody := lipgloss.NewStyle().Foreground(components.UI.MutedTextColor).Render("Console log placeholder")

	leftTop := components.RenderPanel("Installed packages", leftTopBody, leftWidth, leftTopHeight)
	leftBottom := components.RenderPanel(".venv", leftBottomBody, leftWidth, leftBottomHeight)
	rightTop := components.RenderPanel("Description", rightTopBody, rightWidth, rightTopHeight)
	rightBottom := components.RenderPanel("Console log", rightBottomBody, rightWidth, rightBottomHeight)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, leftTop, "", leftBottom)
	rightColumn := lipgloss.JoinVertical(lipgloss.Left, rightTop, "", rightBottom)
	layout := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, strings.Repeat(" ", gap), rightColumn)

	statusLine := lipgloss.NewStyle().Foreground(components.UI.MutedTextColor).Render(m.status)
	fullView := lipgloss.JoinVertical(lipgloss.Left, layout, statusLine)

	// Force full-screen paint to match current terminal size.
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(fullView)
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

func loadPackageDetailsCmd(s appService, packageName string) tea.Cmd {
	return func() tea.Msg {
		if s == nil {
			return packageDetailsLoadedMsg{name: packageName, details: backends.PackageDetails{}, err: nil}
		}
		details, err := s.ShowPackage(context.Background(), packageName)
		return packageDetailsLoadedMsg{name: packageName, details: details, err: err}
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

func (m *AppModel) selectedPackageName() string {
	if len(m.filtered) == 0 {
		return ""
	}
	if m.selected < 0 || m.selected >= len(m.filtered) {
		return ""
	}

	return strings.TrimSpace(m.filtered[m.selected].Name)
}

func (m *AppModel) loadSelectedPackageDetailsCmd() tea.Cmd {
	selected := m.selectedPackageName()
	if selected == "" {
		m.packageDetails = backends.PackageDetails{}
		m.detailsLoading = false
		m.detailsError = ""
		return nil
	}

	m.detailsLoading = true
	m.detailsError = ""
	return loadPackageDetailsCmd(m.service, selected)
}

func (m *AppModel) maybeReloadPackageDetailsCmd(previousSelection string) tea.Cmd {
	if previousSelection == m.selectedPackageName() {
		return nil
	}

	return m.loadSelectedPackageDetailsCmd()
}

