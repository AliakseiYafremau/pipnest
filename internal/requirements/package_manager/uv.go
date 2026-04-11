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

type UVManager struct {
	Binary string
}

func NewUVManager(binary string) *UVManager {
	if strings.TrimSpace(binary) == "" {
		binary = "uv"
	}

	return &UVManager{Binary: binary}
}


// uv pip install <pkg_name>
func (m *UVManager) Install(ctx context.Context, pkgName string) error {
	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	_, err := m.run(ctx, "pip", "install", pkgName)
	return err
}

// uv pip install -r <file_path>
func (m *UVManager) InstallFromFile(ctx context.Context, filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("requirements file path cannot be empty")
	}

	_, err := m.run(ctx, "pip", "install", "-r", filePath)
	return err
}

// uv pip freeze > <file_path>
func (m *UVManager) Freeze(ctx context.Context, filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("output file path cannot be empty")
	}

	out, err := m.run(ctx, "pip", "freeze")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write freeze output to %q: %w", filePath, err)
	}

	return nil
}

// uv pip list --format json
func (m *UVManager) List(ctx context.Context) ([]Dependency, error) {
	out, err := m.run(ctx, "pip", "list", "--format", "json")
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

// NOT IMPLEMENTED
func (m *UVManager) Search(ctx context.Context, query string) ([]Dependency, error) {
	return nil, errors.New("search is not supported by uv")
}

// uv pip uninstall -y <pkgName>
func (m *UVManager) Remove(ctx context.Context, pkgName string) error {
	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	_, err := m.run(ctx, "pip", "uninstall", "-y", pkgName)
	return err
}

// uv run python -c "<code>"
func (m *UVManager) RunPython(ctx context.Context, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", errors.New("python code cannot be empty")
	}

	return m.run(ctx, "run", "python", "-c", code)
}

// Executes the given command
func (m *UVManager) run(ctx context.Context, args ...string) (string, error) {
	binary := m.Binary

	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("run %q with args %v: %s", binary, args, errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}


// Function to parse pip list output
func parsePipTable(out string) []Dependency {
	deps := make([]Dependency, 0)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "package") || strings.HasPrefix(lower, "-") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		deps = append(deps, Dependency{Name: fields[0], Version: fields[1]})
	}

	return deps
}
