package venv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ErrVenvNotFound  = errors.New("venv not found")
	ErrNoCurrentVenv = errors.New("no current venv selected")
)

// Manager is the minimal interface required by higher layers (e.g. service layer)
// for virtual environment operations.
//
// VenvManager implements this interface.
type Manager interface {
	ListVenvs(context.Context) ([]Venv, error)
	CreateVenv(context.Context, VenvCreationStrategy, CreateVenvInput) (Venv, error)
}

// VenvManager is a service-layer orchestrator over backend venv creation strategies.
type VenvManager struct {
	mu      sync.RWMutex
	current *Venv
}

func NewVenvManager() *VenvManager {
	return &VenvManager{}
}

func (m *VenvManager) ListVenvs(_ context.Context) ([]Venv, error) {
	// Discover virtual environments in the current working directory.
	// A directory is considered a venv if it contains either:
	//  - <dir>/bin/python (Unix)
	//  - <dir>/Scripts/python.exe (Windows layout)

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(wd)
	if err != nil {
		return nil, err
	}

	var venvs []Venv
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		root := filepath.Join(wd, e.Name())
		// Only Unix layout for now: look for <root>/bin/python
		unixPython := filepath.Join(root, "bin", "python")
		if _, err := os.Stat(unixPython); err != nil {
			// not a venv (or not Unix-style venv)
			continue
		}

		pythonPath := unixPython

		v := Venv{
			Name:       e.Name(),
			Path:       root,
			PythonPath: pythonPath,
		}
		venvs = append(venvs, v)
	}

	// If manager currently holds a created venv, ensure it's included and placed first.
	m.mu.RLock()
	if m.current != nil {
		found := false
		for _, v := range venvs {
			if EqualsVenv(&v, m.current) {
				found = true
				break
			}
		}
		if !found {
			// prepend current venv
			venvs = append([]Venv{*m.current}, venvs...)
		}
	}
	m.mu.RUnlock()

	return venvs, nil
}

func (m *VenvManager) CreateVenv(ctx context.Context, creator VenvCreationStrategy, input CreateVenvInput) (Venv, error) {
	path := strings.TrimSpace(input.Path)
	if err := creator.handle(ctx, path, strings.TrimSpace(input.PythonPath)); err != nil {
		return Venv{}, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = filepath.Base(path)
	}

	v := Venv{
		Name:       name,
		Path:       path,
		PythonPath: strings.TrimSpace(input.PythonPath),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = &v
	return v, nil
}
