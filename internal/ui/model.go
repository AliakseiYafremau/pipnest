package ui

import (
	"context"
	"github.com/Rotlerxd/pipnest/internal/service"
	"github.com/Rotlerxd/pipnest/internal/ui/components"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type AppModel struct {
	exitKeyMap components.ExitKeyMap
	service    *service.Service
	bindings   []components.Bind
}

func NewAppModel(exitKeyMap components.ExitKeyMap, service *service.Service) *AppModel {
	return &AppModel{
		exitKeyMap: exitKeyMap,
		service:    service,
	}
}

func (m *AppModel) Init() tea.Cmd { return nil }

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.exitKeyMap.Exit) {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *AppModel) View() string {
	// fetch packages from service
	var names []string
	if m.service != nil {
		list, err := m.service.ListPackages(context.Background())
		if err != nil {
			return "Error listing packages: " + err.Error()
		}
		for _, p := range list {
			names = append(names, p.Name)
		}
	} else {
		names = []string{"package1", "package2", "package3"}
	}

	packagesWindow, keys := components.RenderPackagePanel(50, 20, names)
	if keys != nil {
		m.bindings = keys
	}

	return packagesWindow
}
