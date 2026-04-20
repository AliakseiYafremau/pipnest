package components

import (
	bubbles "github.com/charmbracelet/bubbles/key"

	"github.com/charmbracelet/lipgloss"
)

type Bind struct {
	Binding bubbles.Binding
	Handler func()
}

type ExitKeyMap struct {
	Exit bubbles.Binding
}

var StandardExitKeyMap = ExitKeyMap{
	Exit: bubbles.NewBinding(
		bubbles.WithKeys("q", "ctrl+c"),
		bubbles.WithHelp("q/ctrl+c", "quit"),
	),
}

func RenderPanel(title, body string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(UI.BorderColor).
		Padding(UI.PanelPaddingVertical, UI.PanelPaddingHorizontal)

	frameW, frameH := style.GetFrameSize()
	contentWidth := width - frameW
	contentHeight := height - frameH
	if contentWidth < 1 {
		contentWidth = 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	titleStyle := lipgloss.NewStyle().Foreground(UI.TitleColor).Bold(true)
	content := titleStyle.Render(title) + "\n" + body

	return style.
		Width(contentWidth).
		Height(contentHeight).
		Render(content)
}

func RenderRootFrame(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return content
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(UI.BorderColor)

	frameW, frameH := style.GetFrameSize()
	contentWidth := width - frameW
	contentHeight := height - frameH
	if contentWidth < 1 {
		contentWidth = 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	return style.
		Width(contentWidth).
		Height(contentHeight).
		Render(content)
}
