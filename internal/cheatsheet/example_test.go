//go:build linux || darwin
// +build linux darwin

package cheatsheet

import "fmt"

func ExampleFilterCommands() {
	commands := []CheatCommand{
		{Category: "pip", Command: "pip install requests", Description: "install package"},
		{Category: "python", Command: "python --version", Description: "show version"},
	}

	filtered := FilterCommands(commands, "install")
	fmt.Println(len(filtered), filtered[0].Command)
	// Output: 1 pip install requests
}
