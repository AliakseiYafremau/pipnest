//go:build linux || darwin
// +build linux darwin

package venvs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type projectSelectionRecord struct {
	InterpreterPath string          `json:"interpreter_path"`
	Kind            InterpreterKind `json:"kind"`
	UpdatedAt       string          `json:"updated_at"`
}

type projectSelectionFile struct {
	Projects map[string]projectSelectionRecord `json:"projects"`
}

func (m *Model) restoreSelectionFromProject() {
	projectKey, ok := currentProjectKey()
	if !ok {
		return
	}

	record, found := loadProjectInterpreterSelection(projectKey)
	if !found || record.InterpreterPath == "" {
		return
	}

	for idx, option := range m.interpreters {
		if sameInterpreterPath(option.Path, record.InterpreterPath) {
			m.selected = idx
			return
		}
	}
}

func (m *Model) persistSelectionForProject(option InterpreterOption) {
	if option.Path == "" {
		return
	}

	projectKey, ok := currentProjectKey()
	if !ok {
		return
	}

	_ = saveProjectInterpreterSelection(projectKey, projectSelectionRecord{
		InterpreterPath: option.Path,
		Kind:            option.Kind,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	})
}

func currentProjectKey() (string, bool) {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return "", false
	}
	canonical := canonicalPath(wd)
	if canonical == "" {
		canonical = filepath.Clean(wd)
	}
	return canonical, true
}

func sameInterpreterPath(a string, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ca := canonicalPath(a)
	if ca == "" {
		ca = filepath.Clean(a)
	}
	cb := canonicalPath(b)
	if cb == "" {
		cb = filepath.Clean(b)
	}
	return ca == cb
}

func loadProjectInterpreterSelection(projectKey string) (projectSelectionRecord, bool) {
	storePath, ok := selectionStorePath()
	if !ok {
		return projectSelectionRecord{}, false
	}

	payload, err := os.ReadFile(storePath)
	if err != nil {
		return projectSelectionRecord{}, false
	}

	var store projectSelectionFile
	if err := json.Unmarshal(payload, &store); err != nil {
		return projectSelectionRecord{}, false
	}
	if store.Projects == nil {
		return projectSelectionRecord{}, false
	}
	record, exists := store.Projects[projectKey]
	return record, exists
}

func saveProjectInterpreterSelection(projectKey string, record projectSelectionRecord) error {
	storePath, ok := selectionStorePath()
	if !ok {
		return nil
	}

	store := projectSelectionFile{Projects: map[string]projectSelectionRecord{}}
	if payload, err := os.ReadFile(storePath); err == nil {
		_ = json.Unmarshal(payload, &store)
	}
	if store.Projects == nil {
		store.Projects = map[string]projectSelectionRecord{}
	}
	store.Projects[projectKey] = record

	encoded, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return err
	}

	tempPath := storePath + ".tmp"
	if err := os.WriteFile(tempPath, encoded, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, storePath)
}

func selectionStorePath() (string, bool) {
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, "pipnest", "interpreter-selections.json"), true
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".pipnest", "interpreter-selections.json"), true
	}
	return "", false
}
