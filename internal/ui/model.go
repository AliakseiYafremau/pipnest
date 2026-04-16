package ui

import (
	"github.com/Rotlerxd/pipnest/internal/service"
	"github.com/Rotlerxd/pipnest/internal/ui/components"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type AppModel struct {
	exitKeyMap components.ExitKeyMap
	// concrete service pointer (do not change per user's request).
	service *service.Service
	// bindings is the list of key bindings and handlers provided by the
	// package panel. Use key.Matches(msg, bind.Binding) to check activation.
	bindings []components.Bind
}

func NewAppModel(exitKeyMap components.ExitKeyMap, service *service.Service) *AppModel {
	return &AppModel{
		exitKeyMap: exitKeyMap,
		service:    service,
		bindings:   nil,
	}
}

func (m *AppModel) Init() tea.Cmd { return nil }

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If exit key pressed, quit
		if key.Matches(msg, m.exitKeyMap.Exit) {
			return m, tea.Quit
		}
		// Match other bindings provided by the package panel using key.Matches.
		for _, bind := range m.bindings {
			if key.Matches(msg, bind.Binding) {
				if bind.Handler != nil {
					bind.Handler()
				}
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *AppModel) View() string {
	packagesWindow, keys := components.RenderPackagePanel(50, 20, []string{"package1", "package2", "package3"})

	// store bindings for use in Update
	if keys != nil {
		m.bindings = keys
	}

	return packagesWindow
}
