//go:build linux || darwin
// +build linux darwin

package cheatsheet

import "testing"

func TestFilterCommandsByCategory(t *testing.T) {
	commands := []CheatCommand{
		{Category: "pip", Command: "pip install requests", Description: "install package"},
		{Category: "python", Command: "python --version", Description: "show version"},
	}

	filtered := FilterCommands(commands, "PYTHON")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 command, got %d", len(filtered))
	}
	if filtered[0].Command != "python --version" {
		t.Fatalf("unexpected command: %q", filtered[0].Command)
	}
}

func TestFilterCommandsWithEmptySearchReturnsAll(t *testing.T) {
	commands := []CheatCommand{{Command: "a"}, {Command: "b"}}
	filtered := FilterCommands(commands, "")
	if len(filtered) != len(commands) {
		t.Fatalf("expected %d commands, got %d", len(commands), len(filtered))
	}
}
