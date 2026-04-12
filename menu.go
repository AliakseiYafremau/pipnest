//go:build linux || darwin
// +build linux darwin

package main

// ScreenID identifies the app sections to support a future main menu + submenus.
type ScreenID string

const (
	ScreenMainMenu     ScreenID = "main-menu"
	ScreenRequirements ScreenID = "requirements"
	ScreenPackages     ScreenID = "packages"
	ScreenVenvs        ScreenID = "interpreters"
	ScreenCheatSheet   ScreenID = "cheatsheet"
	ScreenEasterEgg    ScreenID = "easter-egg"
)

type MenuItem struct {
	Label  string
	Target ScreenID
}

var MainMenuItems = []MenuItem{
	{Label: "Packages", Target: ScreenRequirements},
	{Label: "Interpreters", Target: ScreenVenvs},
	{Label: "Cheatsheet", Target: ScreenCheatSheet},
}
