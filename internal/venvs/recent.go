//go:build linux || darwin
// +build linux darwin

package venvs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const recentInterpretersEnvVar = "PIPNEST_RECENT_INTERPRETERS_FILE"

type recentInterpreterRecord struct {
	Label     string          `json:"label"`
	Path      string          `json:"path"`
	Root      string          `json:"root"`
	Kind      InterpreterKind `json:"kind"`
	TouchedAt string          `json:"touched_at"`
}

func recentInterpretersPath() string {
	if override := strings.TrimSpace(os.Getenv(recentInterpretersEnvVar)); override != "" {
		return override
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "pipnest", "recent_interpreters.json")
}

func loadRecentInterpreters() []InterpreterOption {
	path := recentInterpretersPath()
	if path == "" {
		return nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var records []recentInterpreterRecord
	if err := json.Unmarshal(payload, &records); err != nil {
		return nil
	}

	options := make([]InterpreterOption, 0, len(records))
	for _, record := range records {
		if record.Path == "" || !fileExists(record.Path) {
			continue
		}
		option := InterpreterOption{
			Label: record.Label,
			Path:  record.Path,
			Root:  record.Root,
			Kind:  record.Kind,
		}
		if option.Kind == "" {
			option.Kind = InterpreterGlobal
		}
		if option.Label == "" {
			option.Label = filepath.Base(option.Path)
		}
		options = append(options, option)
	}
	return options
}

func rememberInterpreter(option InterpreterOption) error {
	if strings.TrimSpace(option.Path) == "" {
		return nil
	}

	path := recentInterpretersPath()
	if path == "" {
		return nil
	}

	records := make([]recentInterpreterRecord, 0, 8)
	if payload, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(payload, &records)
	}

	next := make([]recentInterpreterRecord, 0, len(records)+1)
	next = append(next, recentInterpreterRecord{
		Label:     option.Label,
		Path:      option.Path,
		Root:      option.Root,
		Kind:      option.Kind,
		TouchedAt: time.Now().UTC().Format(time.RFC3339),
	})
	for _, record := range records {
		if record.Path == option.Path {
			continue
		}
		next = append(next, record)
		if len(next) >= 50 {
			break
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func interpreterOptionFromPath(path string) (InterpreterOption, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return InterpreterOption{}, errors.New("interpreter path is required")
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return InterpreterOption{}, fmt.Errorf("invalid interpreter path %q: %w", trimmed, err)
	}
	cleanPath := filepath.Clean(absPath)

	if !fileExists(cleanPath) {
		return InterpreterOption{}, fmt.Errorf("interpreter does not exist: %s", cleanPath)
	}
	if !isExecutableFile(cleanPath) {
		return InterpreterOption{}, fmt.Errorf("interpreter is not executable: %s", cleanPath)
	}

	kind := InterpreterGlobal
	root := ""
	if isVenvLikeInterpreterPath(cleanPath) {
		kind = InterpreterVenv
		root = interpreterRoot(cleanPath)
	}
	if root == "" && kind != InterpreterGlobal {
		root = filepath.Dir(filepath.Dir(cleanPath))
	}

	name := filepath.Base(cleanPath)
	if kind == InterpreterVenv {
		if root != "" {
			name = filepath.Base(root) + " (venv)"
		} else {
			name = filepath.Base(filepath.Dir(cleanPath)) + " (venv)"
		}
	}

	return InterpreterOption{Label: name, Path: cleanPath, Root: root, Kind: kind}, nil
}

func interpreterOptionFromRootPath(root string) (InterpreterOption, error) {
	cleanRoot := strings.TrimSpace(root)
	if cleanRoot == "" {
		return InterpreterOption{}, errors.New("new environment path is required")
	}
	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return InterpreterOption{}, fmt.Errorf("invalid environment path %q: %w", cleanRoot, err)
	}
	name := filepath.Base(absRoot)
	option, ok := interpreterOptionFromRoot(absRoot, name, InterpreterVenv)
	if !ok {
		return InterpreterOption{}, fmt.Errorf("new environment created but interpreter not found under %s", absRoot)
	}
	return option, nil
}
