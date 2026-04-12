package manager

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

type PoetryManager struct {
	Binary string
}

func NewPoetryManager(binary string) *PoetryManager {
	if strings.TrimSpace(binary) == "" {
		binary = "poetry"
	}

	return &PoetryManager{Binary: binary}
}

func (m *PoetryManager) Install(ctx context.Context, pkgName string) error {
	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	_, err := m.run(ctx, "add", pkgName)
	return err
}

func (m *PoetryManager) InstallFromFile(ctx context.Context, filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("requirements file path cannot be empty")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read requirements file %q: %w", filePath, err)
	}

	packages, err := parseRequirementLines(string(content))
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return errors.New("requirements file does not contain any installable packages")
	}

	args := append([]string{"add"}, packages...)
	_, err = m.run(ctx, args...)
	return err
}

func (m *PoetryManager) Freeze(ctx context.Context, filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("output file path cannot be empty")
	}

	out, err := m.run(ctx, "run", "pip", "freeze")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write freeze output to %q: %w", filePath, err)
	}

	return nil
}

func (m *PoetryManager) List(ctx context.Context) ([]Dependency, error) {
	out, err := m.run(ctx, "run", "pip", "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	return parsePackageListJSONOrTable(out)
}

func (m *PoetryManager) Search(ctx context.Context, query string) ([]Dependency, error) {
	return SearchPackages(ctx, query)
}

func (m *PoetryManager) Remove(ctx context.Context, pkgName string) error {
	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	_, err := m.run(ctx, "remove", pkgName)
	return err
}

func (m *PoetryManager) Versions(ctx context.Context, pkgName string) ([]string, error) {
	return NewPipManager("pip").Versions(ctx, pkgName)
}

func (m *PoetryManager) RunPython(ctx context.Context, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", errors.New("python code cannot be empty")
	}

	return m.run(ctx, "run", "python", "-c", code)
}

func (m *PoetryManager) run(ctx context.Context, args ...string) (string, error) {
	binary := strings.TrimSpace(m.Binary)
	if binary == "" {
		binary = "poetry"
	}

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

func parseRequirementLines(content string) ([]string, error) {
	lines := strings.Split(content, "\n")
	packages := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-r ") || strings.HasPrefix(line, "--requirement ") {
			return nil, errors.New("nested requirements files are not supported")
		}
		if strings.HasPrefix(line, "-e ") || strings.HasPrefix(line, "--editable ") {
			return nil, errors.New("editable requirements are not supported")
		}

		packages = append(packages, line)
	}

	return packages, nil
}

func parsePackageListJSONOrTable(out string) ([]Dependency, error) {
	deps, err := parseJSONPackageList(out)
	if err == nil {
		return deps, nil
	}

	return parsePipTable(out), nil
}

func parseJSONPackageList(out string) ([]Dependency, error) {
	parsed, err := decodePackageListJSON(out)
	if err != nil {
		return nil, err
	}

	deps := make([]Dependency, 0, len(parsed))
	for _, pkg := range parsed {
		deps = append(deps, Dependency{Name: pkg.Name, Version: pkg.Version})
	}

	return deps, nil
}

type packageListEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func decodePackageListJSON(out string) ([]packageListEntry, error) {
	var parsed []packageListEntry
	if err := json.Unmarshal([]byte(out), &parsed); err == nil {
		return parsed, nil
	}

	var wrapped struct {
		Packages []packageListEntry `json:"packages"`
	}
	if err := json.Unmarshal([]byte(out), &wrapped); err == nil && len(wrapped.Packages) > 0 {
		return wrapped.Packages, nil
	}

	return nil, errors.New("not json")
}
