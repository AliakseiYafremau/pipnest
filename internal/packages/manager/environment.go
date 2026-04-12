package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CurrentEnvironment struct {
	ProjectPath     string
	InterpreterPath string
	Kind            string
	UpdatedAt       string
}

type interpreterSelectionsFile struct {
	Projects map[string]interpreterSelectionRecord `json:"projects"`
}

type interpreterSelectionRecord struct {
	InterpreterPath string `json:"interpreter_path"`
	Kind            string `json:"kind"`
	UpdatedAt       string `json:"updated_at"`
}

// GetCurrentEnvironment reads ~/.config/pipnest/interpreter-selections.json
// and returns the selected environment for the current working directory.
func GetCurrentEnvironment() (CurrentEnvironment, error) {
	configPath, err := pipnestConfigPath()
	if err != nil {
		return CurrentEnvironment{}, err
	}

	payload, err := os.ReadFile(configPath)
	if err != nil {
		return CurrentEnvironment{}, fmt.Errorf("read pipnest config %q: %w", configPath, err)
	}

	var store interpreterSelectionsFile
	if err := json.Unmarshal(payload, &store); err != nil {
		return CurrentEnvironment{}, fmt.Errorf("parse pipnest config %q: %w", configPath, err)
	}
	if len(store.Projects) == 0 {
		return CurrentEnvironment{}, errors.New("pipnest config does not contain project environments")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return CurrentEnvironment{}, fmt.Errorf("resolve current working directory: %w", err)
	}
	cwdCanonical := canonicalFilesystemPath(cwd)

	if rec, ok := store.Projects[cwd]; ok && strings.TrimSpace(rec.InterpreterPath) != "" {
		return CurrentEnvironment{
			ProjectPath:     cwd,
			InterpreterPath: strings.TrimSpace(rec.InterpreterPath),
			Kind:            strings.TrimSpace(rec.Kind),
			UpdatedAt:       strings.TrimSpace(rec.UpdatedAt),
		}, nil
	}

	for projectPath, rec := range store.Projects {
		if strings.TrimSpace(rec.InterpreterPath) == "" {
			continue
		}
		if canonicalFilesystemPath(projectPath) == cwdCanonical {
			return CurrentEnvironment{
				ProjectPath:     projectPath,
				InterpreterPath: strings.TrimSpace(rec.InterpreterPath),
				Kind:            strings.TrimSpace(rec.Kind),
				UpdatedAt:       strings.TrimSpace(rec.UpdatedAt),
			}, nil
		}
	}

	fallback, ok := latestSelection(store.Projects)
	if !ok {
		return CurrentEnvironment{}, errors.New("current project environment not found in pipnest config")
	}

	return fallback, nil
}

func pipnestConfigPath() (string, error) {
	if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
		return filepath.Join(configDir, "pipnest", "interpreter-selections.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("cannot resolve user config directory")
	}

	return filepath.Join(home, ".config", "pipnest", "interpreter-selections.json"), nil
}

func canonicalFilesystemPath(path string) string {
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err == nil && resolved != "" {
		return filepath.Clean(resolved)
	}
	return clean
}

func latestSelection(projects map[string]interpreterSelectionRecord) (CurrentEnvironment, bool) {
	var chosen CurrentEnvironment
	var chosenTime time.Time
	found := false

	for projectPath, rec := range projects {
		interpreter := strings.TrimSpace(rec.InterpreterPath)
		if interpreter == "" {
			continue
		}

		updatedRaw := strings.TrimSpace(rec.UpdatedAt)
		updatedAt, err := time.Parse(time.RFC3339, updatedRaw)
		if err != nil {
			if !found {
				chosen = CurrentEnvironment{
					ProjectPath:     projectPath,
					InterpreterPath: interpreter,
					Kind:            strings.TrimSpace(rec.Kind),
					UpdatedAt:       updatedRaw,
				}
				found = true
			}
			continue
		}

		if !found || updatedAt.After(chosenTime) {
			chosen = CurrentEnvironment{
				ProjectPath:     projectPath,
				InterpreterPath: interpreter,
				Kind:            strings.TrimSpace(rec.Kind),
				UpdatedAt:       updatedRaw,
			}
			chosenTime = updatedAt
			found = true
		}
	}

	return chosen, found
}

// TouchCurrentEnvironment updates selection timestamp for current project in
// ~/.config/pipnest/interpreter-selections.json.
// If current project is not present, it creates a record using interpreterPath/kind.
func TouchCurrentEnvironment(interpreterPath string, kind string) error {
	configPath, err := pipnestConfigPath()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current working directory: %w", err)
	}
	cwd = filepath.Clean(cwd)
	cwdCanonical := canonicalFilesystemPath(cwd)

	store := interpreterSelectionsFile{Projects: map[string]interpreterSelectionRecord{}}
	if payload, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(payload, &store)
	}
	if store.Projects == nil {
		store.Projects = map[string]interpreterSelectionRecord{}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	recordKey := cwd
	record, found := store.Projects[recordKey]
	if !found {
		for projectPath, rec := range store.Projects {
			if canonicalFilesystemPath(projectPath) == cwdCanonical {
				recordKey = projectPath
				record = rec
				found = true
				break
			}
		}
	}

	if strings.TrimSpace(interpreterPath) == "" {
		interpreterPath = strings.TrimSpace(record.InterpreterPath)
	}
	if strings.TrimSpace(kind) == "" {
		kind = strings.TrimSpace(record.Kind)
	}
	if strings.TrimSpace(interpreterPath) == "" {
		return nil
	}

	store.Projects[recordKey] = interpreterSelectionRecord{
		InterpreterPath: strings.TrimSpace(interpreterPath),
		Kind:            strings.TrimSpace(kind),
		UpdatedAt:       now,
	}

	encoded, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	tmp := configPath + ".tmp"
	if err := os.WriteFile(tmp, encoded, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, configPath)
}
