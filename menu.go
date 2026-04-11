package main

// ScreenID identifies the app sections to support a future main menu + submenus.
type ScreenID string

const (
	ScreenMainMenu     ScreenID = "main-menu"
	ScreenRequirements ScreenID = "requirements"
	ScreenPackages     ScreenID = "packages"
	ScreenVenvs        ScreenID = "venvs"
)

type MenuItem struct {
	Label  string
	Target ScreenID
}

var MainMenuItems = []MenuItem{
	{Label: "Requirements", Target: ScreenRequirements},
	{Label: "Packages", Target: ScreenPackages},
	{Label: "Venvs", Target: ScreenVenvs},
}
