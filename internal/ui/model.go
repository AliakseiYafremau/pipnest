package ui

import (
	"github.com/Rotlerxd/pipnest/internal/service"
	"github.com/Rotlerxd/pipnest/internal/ui/components"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type AppModel struct {
	exitKeyMap components.ExitKeyMap
	service    *service.Service
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
		switch {
		case key.Matches(msg, m.exitKeyMap.Exit):
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *AppModel) View() string {
	packagesWindow := components.RenderPackagePanel(50, 20, []string{"package1", "package2", "package3"})

	return packagesWindow
}
