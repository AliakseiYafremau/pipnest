//go:build linux || darwin
// +build linux darwin

package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// PipManager implements PackageManager with pip commands.
type PipManager struct {
	Binary     string
	PythonPath string
}

// NewPipManager builds a PipManager using binary or the default "pip".
func NewPipManager(binary string) *PipManager {
	if strings.TrimSpace(binary) == "" {
		binary = "pip"
	}

	m := &PipManager{Binary: binary}
	if env, err := GetCurrentEnvironment(); err == nil {
		m.PythonPath = strings.TrimSpace(env.InterpreterPath)
	}

	return m
}

// Install installs a package in the selected interpreter environment.
func (m *PipManager) Install(ctx context.Context, pkgName string) error {
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

// InstallFromFile installs dependencies from a requirements file.
func (m *PipManager) InstallFromFile(ctx context.Context, filePath string) error {
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

// Freeze writes installed dependencies to filePath.
func (m *PipManager) Freeze(ctx context.Context, filePath string) error {
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

// List returns installed dependencies for the active environment.
func (m *PipManager) List(ctx context.Context) ([]Dependency, error) {
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

// Search queries package information from pip.
func (m *PipManager) Search(ctx context.Context, query string) ([]Dependency, error) {
	m.refreshEnvironment()

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	out, err := m.runPip(ctx, "index", "versions", query)
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

// Remove uninstalls a package and prunes removable transitive dependencies.
func (m *PipManager) Remove(ctx context.Context, pkgName string) error {
	m.refreshEnvironment()

	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}

	deps, _ := collectTransitiveDependencies(ctx, pkgName, m.runPip)

	_, err := m.runPip(ctx, "uninstall", "-y", pkgName)
	if err != nil {
		return err
	}

	orphans, err := removableOrphanDependencies(ctx, deps, m.runPip)
	if err == nil && len(orphans) > 0 {
		args := append([]string{"uninstall", "-y"}, orphans...)
		if _, err := m.runPip(ctx, args...); err != nil {
			return err
		}
	}

	_ = TouchCurrentEnvironment(m.PythonPath, "")
	return nil
}

// Versions returns known versions for a package from PyPI.
func (m *PipManager) Versions(ctx context.Context, pkgName string) ([]string, error) {
	m.refreshEnvironment()

	pkgName = strings.TrimSpace(pkgName)
	if pkgName == "" {
		return nil, errors.New("package name cannot be empty")
	}

	versions, err := fetchPyPIVersions(ctx, pkgName)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %q", pkgName)
	}

	return versions, nil
}

// RunPython executes Python code in the selected environment.
func (m *PipManager) RunPython(ctx context.Context, code string) (string, error) {
	m.refreshEnvironment()

	if strings.TrimSpace(code) == "" {
		return "", errors.New("python code cannot be empty")
	}

	if strings.TrimSpace(m.PythonPath) != "" {
		return m.run(ctx, m.PythonPath, "-c", code)
	}

	return m.run(ctx, "python", "-c", code)
}

func (m *PipManager) runPip(ctx context.Context, args ...string) (string, error) {
	if strings.TrimSpace(m.PythonPath) != "" {
		cmd := append([]string{m.Binary, "--python", strings.TrimSpace(m.PythonPath)}, args...)
		return m.run(ctx, cmd...)
	}

	cmd := append([]string{m.Binary}, args...)
	return m.run(ctx, cmd...)
}

func (m *PipManager) refreshEnvironment() {
	if env, err := GetCurrentEnvironment(); err == nil {
		m.PythonPath = strings.TrimSpace(env.InterpreterPath)
	}
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

type pypiReleaseFile struct {
	UploadTimeISO8601 string `json:"upload_time_iso_8601"`
}

type pypiReleaseResponse struct {
	Releases map[string][]pypiReleaseFile `json:"releases"`
}

type versionRow struct {
	Version   string
	Uploaded  time.Time
	HasUpload bool
}

func fetchPyPIVersions(ctx context.Context, pkgName string) ([]string, error) {
	endpoint := fmt.Sprintf("https://pypi.org/pypi/%s/json", url.PathEscape(pkgName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pipnest/1.0")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package %q not found on PyPI", pkgName)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pypi request failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload pypiReleaseResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode pypi response: %w", err)
	}

	rows := make([]versionRow, 0, len(payload.Releases))
	for version, files := range payload.Releases {
		version = strings.TrimSpace(version)
		if version == "" {
			continue
		}

		row := versionRow{Version: version}
		for _, f := range files {
			ts := strings.TrimSpace(f.UploadTimeISO8601)
			if ts == "" {
				continue
			}
			uploaded, parseErr := time.Parse(time.RFC3339Nano, ts)
			if parseErr != nil {
				continue
			}
			if !row.HasUpload || uploaded.After(row.Uploaded) {
				row.Uploaded = uploaded
				row.HasUpload = true
			}
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if cmp := compareVersionStrings(rows[i].Version, rows[j].Version); cmp != 0 {
			return cmp > 0
		}
		if rows[i].HasUpload && rows[j].HasUpload && !rows[i].Uploaded.Equal(rows[j].Uploaded) {
			return rows[i].Uploaded.After(rows[j].Uploaded)
		}
		if rows[i].HasUpload != rows[j].HasUpload {
			return rows[i].HasUpload
		}
		return strings.Compare(rows[i].Version, rows[j].Version) > 0
	})

	versions := make([]string, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, row.Version)
	}

	return versions, nil
}

type versionToken struct {
	isNumber bool
	number   int
	text     string
}

func compareVersionStrings(a string, b string) int {
	aTokens := tokenizeVersion(a)
	bTokens := tokenizeVersion(b)

	maxLen := len(aTokens)
	if len(bTokens) > maxLen {
		maxLen = len(bTokens)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(aTokens) {
			if isPrereleaseToken(bTokens[i].text) {
				return 1
			}
			return -1
		}
		if i >= len(bTokens) {
			if isPrereleaseToken(aTokens[i].text) {
				return -1
			}
			return 1
		}

		at := aTokens[i]
		bt := bTokens[i]

		if at.isNumber && bt.isNumber {
			if at.number > bt.number {
				return 1
			}
			if at.number < bt.number {
				return -1
			}
			continue
		}

		if !at.isNumber && !bt.isNumber {
			if rankA, okA := qualifierRank(at.text); okA {
				if rankB, okB := qualifierRank(bt.text); okB && rankA != rankB {
					if rankA > rankB {
						return 1
					}
					return -1
				}
			}

			cmp := strings.Compare(at.text, bt.text)
			if cmp > 0 {
				return 1
			}
			if cmp < 0 {
				return -1
			}
			continue
		}

		if at.isNumber {
			if isPrereleaseToken(bt.text) {
				return 1
			}
			return -1
		}

		if isPrereleaseToken(at.text) {
			return -1
		}
		return 1
	}

	return 0
}

func tokenizeVersion(version string) []versionToken {
	v := strings.ToLower(strings.TrimSpace(version))
	v = strings.TrimPrefix(v, "v")

	tokens := make([]versionToken, 0, 8)
	var current strings.Builder
	currentIsNumber := false
	hasCurrent := false

	flush := func() {
		if !hasCurrent {
			return
		}
		part := current.String()
		if currentIsNumber {
			n, _ := strconv.Atoi(part)
			tokens = append(tokens, versionToken{isNumber: true, number: n})
		} else {
			tokens = append(tokens, versionToken{text: part})
		}
		current.Reset()
		hasCurrent = false
	}

	for _, r := range v {
		if unicode.IsDigit(r) {
			if hasCurrent && !currentIsNumber {
				flush()
			}
			currentIsNumber = true
			current.WriteRune(r)
			hasCurrent = true
			continue
		}

		if unicode.IsLetter(r) {
			if hasCurrent && currentIsNumber {
				flush()
			}
			currentIsNumber = false
			current.WriteRune(r)
			hasCurrent = true
			continue
		}

		flush()
	}

	flush()
	return tokens
}

func qualifierRank(token string) (int, bool) {
	switch token {
	case "dev":
		return 0, true
	case "a", "alpha":
		return 1, true
	case "b", "beta":
		return 2, true
	case "rc", "c", "pre", "preview":
		return 3, true
	case "post", "rev", "r":
		return 5, true
	default:
		return 0, false
	}
}

func isPrereleaseToken(token string) bool {
	_, ok := qualifierRank(token)
	if !ok {
		return false
	}
	return token != "post" && token != "rev" && token != "r"
}
