package requirements

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type PipManager struct {
	Binary string
}

func NewPipManager(binary string) *PipManager {
	if strings.TrimSpace(binary) == "" {
		binary = "pip"
	}

	return &PipManager{Binary: binary}
}

func (m *PipManager) CreateVenv(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("venv name cannot be empty")
	}

	_, err := m.run(ctx, "python", "-m", "venv", name)
	return err
}

func (m *PipManager) Install(ctx context.Context, pkgName string) error {
	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	_, err := m.run(ctx, m.Binary, "install", pkgName)
	return err
}

func (m *PipManager) InstallFromFile(ctx context.Context, filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("requirements file path cannot be empty")
	}

	_, err := m.run(ctx, m.Binary, "install", "-r", filePath)
	return err
}

func (m *PipManager) Freeze(ctx context.Context, filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("output file path cannot be empty")
	}

	out, err := m.run(ctx, m.Binary, "freeze")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write freeze output to %q: %w", filePath, err)
	}

	return nil
}

func (m *PipManager) List(ctx context.Context) ([]Dependency, error) {
	out, err := m.run(ctx, m.Binary, "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	type pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	var parsed []pkg
	if err := json.Unmarshal([]byte(out), &parsed); err == nil {
		deps := make([]Dependency, 0, len(parsed))
		for _, p := range parsed {
			deps = append(deps, Dependency{Name: p.Name, Version: p.Version})
		}
		return deps, nil
	}

	return parsePipTable(out), nil
}

func (m *PipManager) Search(ctx context.Context, query string) ([]Dependency, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	out, err := m.run(ctx, m.Binary, "index", "versions", query)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Expected format: <name> (<latest>)
		open := strings.Index(line, "(")
		close := strings.LastIndex(line, ")")
		if open <= 0 || close <= open {
			continue
		}

		name := strings.TrimSpace(line[:open])
		ver := strings.TrimSpace(line[open+1 : close])
		if name != "" {
			return []Dependency{{Name: name, Version: ver}}, nil
		}
	}

	return []Dependency{{Name: query, Version: ""}}, nil
}

func (m *PipManager) Remove(ctx context.Context, pkgName string) error {
	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	_, err := m.run(ctx, m.Binary, "uninstall", "-y", pkgName)
	return err
}

func (m *PipManager) RunPython(ctx context.Context, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", errors.New("python code cannot be empty")
	}

	return m.run(ctx, "python", "-c", code)
}

func (m *PipManager) run(ctx context.Context, args ...string) (string, error) {
	binary := args[0]
	cmdArgs := args[1:]

	cmd := exec.CommandContext(ctx, binary, cmdArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("run %q with args %v: %s", binary, cmdArgs, errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}
