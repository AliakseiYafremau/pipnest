package requirements

import (
	"context"
	"fmt"
	packagemanager "pipnest/internal/requirements/package_manager"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ViewModel struct {
	Width          int
	Height         int
	PackageManager packagemanager.PackageManager

	Input      textinput.Model
	Query      string
	Packages   []packagemanager.Dependency
	Selected   int
	Scroll     int
	Loading    bool
	Installing bool
	Status     string
	Err        error
}

func NewViewModel() ViewModel {
	ti := textinput.New()
	ti.Placeholder = "Search packages..."
	ti.Focus()

	return ViewModel{
		PackageManager: packagemanager.NewUVManager("uv"),
		Input:          ti,
	}
}

func (m ViewModel) Init() tea.Cmd {
	return textinput.Blink
}

type searchDoneMsg struct {
	Query   string
	Results []packagemanager.Dependency
	Err     error
}

type installDoneMsg struct {
	Name string
	Err  error
}

func (m ViewModel) Update(msg tea.Msg) (ViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.ensureSelectionVisible(m.visibleResultRows())
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyCtrlP:
			if m.Selected > 0 {
				m.Selected--
				m.ensureSelectionVisible(m.visibleResultRows())
			}
			return m, nil
		case tea.KeyDown, tea.KeyCtrlN:
			if m.Selected < len(m.Packages)-1 {
				m.Selected++
				m.ensureSelectionVisible(m.visibleResultRows())
			}
			return m, nil
		case tea.KeyPgUp:
			step := m.visibleResultRows()
			if step < 1 {
				step = 1
			}
			m.Selected -= step
			if m.Selected < 0 {
				m.Selected = 0
			}
			m.ensureSelectionVisible(m.visibleResultRows())
			return m, nil
		case tea.KeyPgDown:
			step := m.visibleResultRows()
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
			m.ensureSelectionVisible(m.visibleResultRows())
			return m, nil
		case tea.KeyEnter:
			query := strings.TrimSpace(m.Input.Value())
			if query == "" {
				m.Query = ""
				m.Packages = nil
				m.Selected = 0
				m.Scroll = 0
				m.Err = nil
				m.Status = ""
				m.Loading = false
				return m, nil
			}

			m.Query = query
			m.Loading = true
			m.Err = nil
			m.Status = "Searching..."
			return m, m.searchCmd(query)
		case tea.KeyRunes:
			if len(msg.Runes) == 1 && (msg.Runes[0] == 'i' || msg.Runes[0] == 'I') {
				if m.Installing || len(m.Packages) == 0 || m.Selected < 0 || m.Selected >= len(m.Packages) {
					return m, nil
				}

				name := strings.TrimSpace(m.Packages[m.Selected].Name)
				if name == "" {
					return m, nil
				}

				m.Installing = true
				m.Err = nil
				m.Status = fmt.Sprintf("Installing %s...", name)
				return m, m.installCmd(name)
			}
		}
	case searchDoneMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			m.Packages = nil
			m.Selected = 0
			m.Scroll = 0
			m.Status = "Search failed"
			return m, nil
		}

		m.Packages = msg.Results
		m.Selected = 0
		m.Scroll = 0
		m.Err = nil
		m.Status = fmt.Sprintf("Found %d packages", len(msg.Results))
		return m, nil
	case installDoneMsg:
		m.Installing = false
		if msg.Err != nil {
			m.Err = msg.Err
			m.Status = "Install failed"
			return m, nil
		}

		m.Err = nil
		m.Status = fmt.Sprintf("Installed %s", msg.Name)
		return m, nil
	}

	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)
	return m, cmd
}

func (m ViewModel) View() string {
	if m.Width == 0 || m.Height == 0 {
		return ""
	}

	topHeight := 4
	contentHeight := m.Height - topHeight - 1
	if contentHeight < 8 {
		contentHeight = 8
	}

	leftWidth := (m.Width - 3) / 2
	if leftWidth < 28 {
		leftWidth = 28
	}
	rightWidth := m.Width - 3 - leftWidth
	if rightWidth < 28 {
		rightWidth = 28
	}

	inputStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(m.Width - 2).Height(topHeight - 2)
	leftStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(leftWidth).Height(contentHeight - 2)
	rightStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(rightWidth).Height(contentHeight - 2)

	status := "Enter to search, i to install selected package"
	if m.Loading {
		status = "Searching..."
	} else if m.Installing {
		status = "Installing package..."
	} else if m.Status != "" {
		status = m.Status
	}
	if m.Err != nil {
		status = "Error: " + m.Err.Error()
	}

	top := inputStyle.Render(strings.Join([]string{m.Input.View(), status}, "\n"))
	leftBody := m.renderPackageInfo(leftWidth - 4)
	rightBody := m.renderPackageList(rightWidth-4, contentHeight-4)

	leftPane := leftStyle.Render(leftBody)
	rightPane := rightStyle.Render(rightBody)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, lipgloss.NewStyle().Width(1).Render("│"), rightPane)

	return top + "\n" + bottom
}

func (m ViewModel) searchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		results, err := m.PackageManager.Search(ctx, query)
		return searchDoneMsg{Query: query, Results: results, Err: err}
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

func (m *ViewModel) ensureSelectionVisible(visibleRows int) {
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

func (m ViewModel) visibleResultRows() int {
	rows := m.Height - 8
	if rows < 5 {
		rows = 5
	}
	return rows
}

func (m ViewModel) renderPackageList(width int, rows int) string {
	if width < 10 {
		width = 10
	}
	if rows < 3 {
		rows = 3
	}

	header := lipgloss.NewStyle().Bold(true).Render("Packages")
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("Up/Down/PgUp/PgDn to navigate")

	if len(m.Packages) == 0 {
		if m.Loading {
			return strings.Join([]string{header, hint, "", "Loading results..."}, "\n")
		}
		return strings.Join([]string{header, hint, "", "No packages yet. Search above."}, "\n")
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

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("57"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	lines := []string{header, hint, ""}
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

	info := fmt.Sprintf("%d/%d", m.Selected+1, len(m.Packages))
	if len(m.Packages) == 0 {
		info = "0/0"
	}
	lines = append(lines, "", mutedStyle.Render(info))

	return strings.Join(lines, "\n")
}

func (m ViewModel) renderPackageInfo(width int) string {
	if width < 10 {
		width = 10
	}

	header := lipgloss.NewStyle().Bold(true).Render("Package Info")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true)

	lines := []string{header, muted.Render(strings.TrimSpace(m.Query)), ""}
	if len(m.Packages) == 0 || m.Selected < 0 || m.Selected >= len(m.Packages) {
		lines = append(lines, muted.Render("Select a package from the right panel."))
		lines = append(lines, "")
		lines = append(lines, muted.Render("Install: press i"))
		return strings.Join(lines, "\n")
	}

	dep := m.Packages[m.Selected]
	lines = append(lines, accent.Render(dep.Name))
	lines = append(lines, muted.Render("Version"))
	ver := dep.Version
	if strings.TrimSpace(ver) == "" {
		ver = "unknown"
	}
	lines = append(lines, WrapText(ver, width))
	lines = append(lines, "")
	lines = append(lines, muted.Render("Install selected package: press i"))

	return strings.Join(lines, "\n")
}
