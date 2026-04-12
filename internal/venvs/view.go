//go:build linux || darwin
// +build linux darwin

package venvs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	labelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	globalStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B8BBE"))
	virtualEnvStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffde57"))
)

// ViewModel renders a compact interpreter status card.
type ViewModel struct {
	Width           int
	Height          int
	Interpreter     string
	InterpreterKind InterpreterKind
}

// NewViewModel builds a status view from the detected interpreter.
func NewViewModel() ViewModel {
	interpreter, kind := DetectInterpreter()
	return ViewModel{
		Interpreter:     interpreter,
		InterpreterKind: kind,
	}
}

// View renders the interpreter status card.
func (m ViewModel) View() string {
	if m.Width <= 0 || m.Height <= 0 {
		return ""
	}

	interpreter := m.Interpreter
	kind := m.InterpreterKind
	if interpreter == "" {
		interpreter = "No active Python interpreter detected"
		kind = InterpreterGlobal
	}

	title := labelStyle.Render("Interprete actual")
	contentStyle := StyleForInterpreter(kind)
	content := fmt.Sprintf("%s\n%s", title, contentStyle.Render(interpreter))
	innerWidth := lipgloss.Width(interpreter)
	titleWidth := lipgloss.Width("Interprete actual")
	if titleWidth > innerWidth {
		innerWidth = titleWidth
	}
	innerWidth += 4
	if innerWidth > m.Width-4 {
		innerWidth = m.Width - 4
		if innerWidth < 20 {
			innerWidth = 20
		}
		interpreter = truncateLine(interpreter, innerWidth-4)
		content = fmt.Sprintf("%s\n%s", title, contentStyle.Render(interpreter))
	}

	box := boxStyleForInterpreter(kind).
		Padding(0, 2).
		Width(innerWidth).
		Render(content)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, box)
}

// StyleForInterpreter returns the lipgloss style for the given interpreter kind.
func StyleForInterpreter(kind InterpreterKind) lipgloss.Style {
	switch kind {
	case InterpreterVenv, InterpreterConda:
		return virtualEnvStyle
	default:
		return globalStyle
	}
}

func boxStyleForInterpreter(kind InterpreterKind) lipgloss.Style {
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	switch kind {
	case InterpreterVenv, InterpreterConda:
		return border.BorderForeground(lipgloss.Color("#ffde57"))
	default:
		return border.BorderForeground(lipgloss.Color("#4B8BBE"))
	}
}

func truncateLine(value string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= maxWidth {
		return value
	}

	ellipsis := "..."
	if maxWidth <= len(ellipsis) {
		return strings.Repeat(".", maxWidth)
	}

	remaining := maxWidth - len(ellipsis)
	runes := []rune(value)
	if remaining >= len(runes) {
		return value
	}

	left := remaining / 2
	right := remaining - left
	return string(runes[:left]) + ellipsis + string(runes[len(runes)-right:])
}
