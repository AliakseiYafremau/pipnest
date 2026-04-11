package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pipnest/internal/pkgsearch"
)

type searchResult = pkgsearch.Result

type searchDoneMsg = pkgsearch.DoneMsg

type model struct {
	input    textinput.Model
	width    int
	height   int
	query    string
	results  []searchResult
	selected int
	loading  bool
	err      error
}

const (
	topInputHeight       = 5
	resultMouseStartLine = topInputHeight + 5
)

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Search PyPI packages..."
	ti.Focus()

	return model{input: ti}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
			return m, tea.Quit
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
			return m, pkgsearch.Search(query)
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

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	inputHeight := topInputHeight
	contentHeight := m.height - inputHeight - 1
	if contentHeight < 10 {
		contentHeight = 10
	}

	leftPaneWidth := (m.width - 3) / 2
	if leftPaneWidth < 24 {
		leftPaneWidth = 24
	}
	rightPaneWidth := m.width - 3 - leftPaneWidth
	if rightPaneWidth < 24 {
		rightPaneWidth = 24
	}

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(m.width - 2).
		Height(inputHeight - 2)

	leftStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(leftPaneWidth).
		Height(contentHeight - 2)

	rightStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(rightPaneWidth).
		Height(contentHeight - 2)

	status := "Press Enter to search"
	if m.loading {
		status = "Searching..."
	} else if m.query != "" {
		status = fmt.Sprintf("Results for %q", m.query)
	}
	if m.err != nil {
		status = "Search error: " + m.err.Error()
	}

	inputBody := strings.Join([]string{m.input.View(), status}, "\n")
	resultsBody := pkgsearch.RenderResults(m.results, leftPaneWidth-4, m.selected)
	if resultsBody == "" {
		if m.loading {
			resultsBody = "Loading results..."
		} else {
			resultsBody = "Type a package name and press Enter."
		}
	}
	selectedResult := pkgsearch.SelectedSearchResult(m.results, m.selected)
	rightBody := pkgsearch.RenderPackageDetails(selectedResult, rightPaneWidth-4, m.loading, m.query, m.err)

	top := inputStyle.Render(inputBody)
	leftPane := leftStyle.Render(resultsBody)
	rightPane := rightStyle.Render(rightBody)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, lipgloss.NewStyle().Width(1).Render("│"), rightPane)

	return top + "\n" + bottom
}
