package venvs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListInterpretersFindsLocalVenvInPwd(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()

	tempDir := t.TempDir()
	localVenv := filepath.Join(tempDir, ".venv")
	pythonPath := filepath.Join(localVenv, "bin", "python")
	if err := os.MkdirAll(filepath.Dir(pythonPath), 0o755); err != nil {
		t.Fatalf("create local venv: %v", err)
	}
	if err := os.WriteFile(pythonPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write local python: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	options := ListInterpreters()
	if len(options) == 0 {
		t.Fatal("expected at least one interpreter")
	}
	if options[0].Kind != InterpreterVenv {
		t.Fatalf("expected local venv first, got kind %q", options[0].Kind)
	}
	if options[0].Path != pythonPath {
		t.Fatalf("expected local venv path %q, got %q", pythonPath, options[0].Path)
	}
}

func TestInterpreterOptionActivationCommandForVenv(t *testing.T) {
	tempVenv := t.TempDir()
	activatePath := filepath.Join(tempVenv, "bin", "activate")
	if err := os.MkdirAll(filepath.Dir(activatePath), 0o755); err != nil {
		t.Fatalf("create activate dir: %v", err)
	}
	if err := os.WriteFile(activatePath, []byte(""), 0o644); err != nil {
		t.Fatalf("write activate script: %v", err)
	}

	command := InterpreterOption{
		Root: tempVenv,
		Kind: InterpreterVenv,
	}.ActivationCommand()

	expected := "source '" + activatePath + "'"
	if command != expected {
		t.Fatalf("expected %q, got %q", expected, command)
	}
}

func TestInterpreterOptionDetailsIncludeTimestampLabels(t *testing.T) {
	tempVenv := t.TempDir()
	pythonPath := filepath.Join(tempVenv, "bin", "python")
	if err := os.MkdirAll(filepath.Dir(pythonPath), 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.WriteFile(pythonPath, []byte(""), 0o644); err != nil {
		t.Fatalf("create fake python: %v", err)
	}

	details := InterpreterOption{
		Path: pythonPath,
		Root: tempVenv,
		Kind: InterpreterVenv,
	}.Details()

	if details.CreatedAtLabel == "" {
		t.Fatal("expected created timestamp label")
	}
	if details.UpdatedAtLabel == "" {
		t.Fatal("expected updated timestamp label")
	}

	createdAt, err := time.Parse("2006-01-02 15:04", details.CreatedAtLabel)
	if err != nil {
		t.Fatalf("parse created timestamp: %v", err)
	}
	updatedAt, err := time.Parse("2006-01-02 15:04", details.UpdatedAtLabel)
	if err != nil {
		t.Fatalf("parse updated timestamp: %v", err)
	}
	if updatedAt.Before(createdAt) {
		t.Fatalf("expected updated timestamp >= created timestamp, got created=%q updated=%q", details.CreatedAtLabel, details.UpdatedAtLabel)
	}
}
