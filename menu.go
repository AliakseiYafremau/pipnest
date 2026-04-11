package main

// ScreenID identifies the app sections to support a future main menu + submenus.
type ScreenID string

const (
	ScreenMainMenu     ScreenID = "main-menu"
	ScreenRequirements ScreenID = "requirements"
	ScreenPackages     ScreenID = "packages"
	ScreenVenvs        ScreenID = "venvs"
	ScreenCheatSheet   ScreenID = "cheatsheet"
)

type MenuItem struct {
	Label  string
	Target ScreenID
}

var MainMenuItems = []MenuItem{
	{Label: "Requirements", Target: ScreenRequirements},
	{Label: "Venvs", Target: ScreenVenvs},
	{Label: "Cheatsheet", Target: ScreenCheatSheet},
}
