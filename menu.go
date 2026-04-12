//go:build linux || darwin
// +build linux darwin

package main

// ScreenID identifies the app sections to support a future main menu + submenus.
type ScreenID string

const (
	// ScreenMainMenu is the top-level landing screen.
	ScreenMainMenu ScreenID = "main-menu"
	// ScreenRequirements opens installed dependency management.
	ScreenRequirements ScreenID = "requirements"
	// ScreenPackages opens the PyPI search view.
	ScreenPackages ScreenID = "packages"
	// ScreenVenvs opens interpreter and virtual environment management.
	ScreenVenvs ScreenID = "interpreters"
	// ScreenCheatSheet opens the Python command cheatsheet.
	ScreenCheatSheet ScreenID = "cheatsheet"
	// ScreenEasterEgg opens the hidden easter-egg screen.
	ScreenEasterEgg ScreenID = "easter-egg"
)

// MenuItem maps a menu label to its destination screen.
type MenuItem struct {
	Label  string
	Target ScreenID
}

// MainMenuItems defines the available entries shown in the main menu.
var MainMenuItems = []MenuItem{
	{Label: "Packages", Target: ScreenRequirements},
	{Label: "Interpreters", Target: ScreenVenvs},
	{Label: "Cheatsheet", Target: ScreenCheatSheet},
}
