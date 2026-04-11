package venvs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRememberAndLoadRecentInterpreters(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "recent.json")
	t.Setenv(recentInterpretersEnvVar, storePath)

	pythonPath := filepath.Join(tempDir, "project", ".venv", "bin", "python")
	if err := os.MkdirAll(filepath.Dir(pythonPath), 0o755); err != nil {
		t.Fatalf("create path: %v", err)
	}
	if err := os.WriteFile(pythonPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write interpreter: %v", err)
	}

	option := InterpreterOption{Label: ".venv (venv)", Path: pythonPath, Root: filepath.Dir(filepath.Dir(pythonPath)), Kind: InterpreterVenv}
	if err := rememberInterpreter(option); err != nil {
		t.Fatalf("remember interpreter: %v", err)
	}

	loaded := loadRecentInterpreters()
	if len(loaded) != 1 {
		t.Fatalf("expected one recent interpreter, got %d", len(loaded))
	}
	if loaded[0].Path != pythonPath {
		t.Fatalf("expected path %q, got %q", pythonPath, loaded[0].Path)
	}
}

func TestLoadRecentInterpretersSkipsMissingPaths(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "recent.json")
	t.Setenv(recentInterpretersEnvVar, storePath)

	records := []recentInterpreterRecord{{Label: "missing", Path: filepath.Join(tempDir, "missing", "python"), Kind: InterpreterVenv}}
	payload, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal records: %v", err)
	}
	if err := os.WriteFile(storePath, payload, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}

	loaded := loadRecentInterpreters()
	if len(loaded) != 0 {
		t.Fatalf("expected no interpreters loaded, got %d", len(loaded))
	}
}

func TestInterpreterOptionFromPathPreservesVenvSymlinkPath(t *testing.T) {
	tempDir := t.TempDir()
	venvRoot := filepath.Join(tempDir, ".venv")
	pythonPath := filepath.Join(venvRoot, "bin", "python")
	if err := os.MkdirAll(filepath.Dir(pythonPath), 0o755); err != nil {
		t.Fatalf("create venv bin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(venvRoot, "pyvenv.cfg"), []byte("home=/usr/bin\n"), 0o644); err != nil {
		t.Fatalf("create pyvenv.cfg: %v", err)
	}
	if err := os.Symlink("/bin/sh", pythonPath); err != nil {
		t.Fatalf("create python symlink: %v", err)
	}

	option, err := interpreterOptionFromPath(pythonPath)
	if err != nil {
		t.Fatalf("interpreter option from path: %v", err)
	}
	if option.Kind != InterpreterVenv {
		t.Fatalf("expected kind %q, got %q", InterpreterVenv, option.Kind)
	}
	if option.Path != pythonPath {
		t.Fatalf("expected path %q, got %q", pythonPath, option.Path)
	}
	if option.Root != venvRoot {
		t.Fatalf("expected root %q, got %q", venvRoot, option.Root)
	}
}
