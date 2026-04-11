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
	Binary     string
	PythonPath string
	fallback   PackageManager
}

func NewUVManager(binary string) *UVManager {
	if strings.TrimSpace(binary) == "" {
		binary = "uv"
	}

	m := &UVManager{Binary: binary}
	if _, err := exec.LookPath(binary); err != nil {
		m.fallback = NewPipManager("pip")
	}
	if env, err := GetCurrentEnvironment(); err == nil {
		m.PythonPath = strings.TrimSpace(env.InterpreterPath)
	}

	return m
}

// Install package into selected environment.
func (m *UVManager) Install(ctx context.Context, pkgName string) error {
	if m.fallback != nil {
		return m.fallback.Install(ctx, pkgName)
	}

	m.refreshEnvironment()

	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	_, err := m.runPip(ctx, "install", pkgName)
	if err != nil {
		return err
	}

	_ = TouchCurrentEnvironment(m.PythonPath, "")
	return nil
}

// Install package set from requirements file into selected environment.
func (m *UVManager) InstallFromFile(ctx context.Context, filePath string) error {
	if m.fallback != nil {
		return m.fallback.InstallFromFile(ctx, filePath)
	}

	m.refreshEnvironment()

	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("requirements file path cannot be empty")
	}

	_, err := m.runPip(ctx, "install", "-r", filePath)
	if err != nil {
		return err
	}

	_ = TouchCurrentEnvironment(m.PythonPath, "")
	return nil
}

// Freeze selected environment requirements into a file.
func (m *UVManager) Freeze(ctx context.Context, filePath string) error {
	if m.fallback != nil {
		return m.fallback.Freeze(ctx, filePath)
	}

	m.refreshEnvironment()

	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("output file path cannot be empty")
	}

	out, err := m.runPip(ctx, "freeze")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write freeze output to %q: %w", filePath, err)
	}

	return nil
}

// List installed packages in selected environment.
func (m *UVManager) List(ctx context.Context) ([]Dependency, error) {
	if m.fallback != nil {
		return m.fallback.List(ctx)
	}

	m.refreshEnvironment()

	out, err := m.runPip(ctx, "list", "--format", "json")
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

func (m *UVManager) Search(ctx context.Context, query string) ([]Dependency, error) {
	if m.fallback != nil {
		return m.fallback.Search(ctx, query)
	}

	m.refreshEnvironment()

	return SearchPackages(ctx, query)
}

// Remove package from selected environment.
func (m *UVManager) Remove(ctx context.Context, pkgName string) error {
	if m.fallback != nil {
		return m.fallback.Remove(ctx, pkgName)
	}

	m.refreshEnvironment()

	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	_, err := m.runPip(ctx, "uninstall", pkgName)
	return err
}

// uv run python -c "<code>"
func (m *UVManager) RunPython(ctx context.Context, code string) (string, error) {
	if m.fallback != nil {
		return m.fallback.RunPython(ctx, code)
	}

	m.refreshEnvironment()

	if strings.TrimSpace(code) == "" {
		return "", errors.New("python code cannot be empty")
	}

	args := []string{"run"}
	if strings.TrimSpace(m.PythonPath) != "" {
		args = append(args, "--python", strings.TrimSpace(m.PythonPath))
	}
	args = append(args, "python", "-c", code)
	return m.run(ctx, args...)
}

func (m *UVManager) runPip(ctx context.Context, args ...string) (string, error) {
	uvArgs := []string{"pip"}
	if len(args) > 0 {
		subcommand := args[0]
		uvArgs = append(uvArgs, subcommand)
		if strings.TrimSpace(m.PythonPath) != "" {
			uvArgs = append(uvArgs, "--python", strings.TrimSpace(m.PythonPath))
		}
		if len(args) > 1 {
			uvArgs = append(uvArgs, args[1:]...)
		}
		return m.run(ctx, uvArgs...)
	}

	if strings.TrimSpace(m.PythonPath) != "" {
		uvArgs = append(uvArgs, "--python", strings.TrimSpace(m.PythonPath))
	}

	return m.run(ctx, uvArgs...)
}

func (m *UVManager) refreshEnvironment() {
	if env, err := GetCurrentEnvironment(); err == nil {
		m.PythonPath = strings.TrimSpace(env.InterpreterPath)
	}
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
