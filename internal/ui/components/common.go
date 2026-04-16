package components

import bubbles "github.com/charmbracelet/bubbles/key"

type ExitKeyMap struct {
	Exit bubbles.Binding
}

var StandardExitKeyMap = ExitKeyMap{
	Exit: bubbles.NewBinding(
		bubbles.WithKeys("q", "ctrl+c"),
		bubbles.WithKeys("ctrl+c")),
}
