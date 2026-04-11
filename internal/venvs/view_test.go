package venvs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestViewRendersInterpreterBox(t *testing.T) {
	tempVenv := t.TempDir()
	pythonPath := filepath.Join(tempVenv, "bin", "python")
	if err := os.MkdirAll(filepath.Dir(pythonPath), 0o755); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	if err := os.WriteFile(pythonPath, []byte(""), 0o644); err != nil {
		t.Fatalf("create fake python: %v", err)
	}

	t.Setenv("VIRTUAL_ENV", tempVenv)
	t.Setenv("CONDA_PREFIX", "")

	model := NewViewModel()
	model.Width = 80
	model.Height = 24

	view := model.View()
	if view == "" {
		t.Fatal("expected a rendered view")
	}
	if !strings.Contains(view, "Interprete actual") {
		t.Fatalf("expected title in view, got:\n%s", view)
	}
	if !strings.Contains(view, filepath.Base(tempVenv)) {
		t.Fatalf("expected venv name in view, got:\n%s", view)
	}

	t.Logf("rendered view:\n%s", view)
}

func TestInterpreterStylesDiffer(t *testing.T) {
	global := styleForInterpreter(InterpreterGlobal).Render("python")
	venv := styleForInterpreter(InterpreterVenv).Render("python")

	if global == venv {
		t.Fatal("expected global and venv styles to render differently")
	}
}
