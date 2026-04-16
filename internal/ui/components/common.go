package components

import bubbles "github.com/charmbracelet/bubbles/key"

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

