package venvs

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type InterpreterKind string

const (
	InterpreterGlobal InterpreterKind = "global"
	InterpreterVenv   InterpreterKind = "venv"
	InterpreterConda  InterpreterKind = "conda"
)

type InterpreterOption struct {
	Label string
	Path  string
	Root  string
	Kind  InterpreterKind
}

type PackageInfo struct {
	Name    string
	Version string
}

type InterpreterDetails struct {
	Path              string
	Kind              InterpreterKind
	Version           string
	SizeLabel         string
	CreatedAtLabel    string
	UpdatedAtLabel    string
	PackageCount      int
	Packages          []PackageInfo
	ActivationCommand string
}

func (option InterpreterOption) ActivationCommand() string {
	switch option.Kind {
	case InterpreterVenv:
		activatePath := option.activatePath()
		if activatePath == "" {
			return ""
		}
		return fmt.Sprintf("source %s", shellQuote(activatePath))
	case InterpreterConda:
		if option.Root == "" {
			return ""
		}
		return fmt.Sprintf("conda activate %s", shellQuote(option.Root))
	default:
		return ""
	}
}

func (option InterpreterOption) Details() InterpreterDetails {
	details := InterpreterDetails{
		Path:              option.Path,
		Kind:              option.Kind,
		ActivationCommand: option.ActivationCommand(),
	}
	details.Version = option.pythonVersion()
	details.SizeLabel = option.sizeLabel()
	details.CreatedAtLabel, details.UpdatedAtLabel = option.timestampLabels()
	details.Packages = option.installedPackages()
	details.PackageCount = len(details.Packages)
	return details
}

func (option InterpreterOption) timestampLabels() (string, string) {
	targetPath := option.metadataPath()
	if targetPath == "" {
		return "", ""
	}

	created, updated, ok := fileTimeRange(targetPath)
	if !ok {
		return "", ""
	}

	return formatTimestamp(created), formatTimestamp(updated)
}

func (option InterpreterOption) metadataPath() string {
	if option.Kind == InterpreterGlobal {
		return option.Path
	}
	if option.Root != "" {
		return option.Root
	}
	if option.Path == "" {
		return ""
	}
	return filepath.Dir(filepath.Dir(option.Path))
}

func fileTimeRange(path string) (time.Time, time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}

	created := info.ModTime()
	updated := created

	if !info.IsDir() {
		return created, updated, true
	}

	_ = filepath.WalkDir(path, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		entryInfo, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		modifiedAt := entryInfo.ModTime()
		if modifiedAt.Before(created) {
			created = modifiedAt
		}
		if modifiedAt.After(updated) {
			updated = modifiedAt
		}
		return nil
	})

	return created, updated, true
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04")
}

func DetectInterpreter() (string, InterpreterKind) {
	options := ListInterpreters()
	if len(options) == 0 {
		return "", InterpreterGlobal
	}

	return options[0].Path, options[0].Kind
}

func ListInterpreters() []InterpreterOption {
	options := make([]InterpreterOption, 0, 4)
	seen := make(map[string]struct{})

	add := func(option InterpreterOption) {
		if option.Path == "" {
			return
		}
		if _, exists := seen[option.Path]; exists {
			return
		}
		seen[option.Path] = struct{}{}
		options = append(options, option)
	}

	addLocalVenvs(add)
	addActiveVenv(add)

	if condaPrefix := os.Getenv("CONDA_PREFIX"); condaPrefix != "" {
		name := filepath.Base(condaPrefix)
		if runtimeInterpreter := filepath.Join(condaPrefix, "bin", "python"); fileExists(runtimeInterpreter) {
			add(InterpreterOption{Label: name + " (conda)", Path: runtimeInterpreter, Root: condaPrefix, Kind: InterpreterConda})
		} else if runtimeInterpreter := filepath.Join(condaPrefix, "Scripts", "python.exe"); fileExists(runtimeInterpreter) {
			add(InterpreterOption{Label: name + " (conda)", Path: runtimeInterpreter, Root: condaPrefix, Kind: InterpreterConda})
		}
	}

	for _, option := range globalInterpretersFromPath() {
		add(option)
	}

	return options
}

func globalInterpretersFromPath() []InterpreterOption {
	pathValue := os.Getenv("PATH")
	if pathValue == "" {
		return nil
	}

	collected := make([]InterpreterOption, 0, 8)
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			continue
		}
		if isWslEnvironment() && strings.HasPrefix(filepath.ToSlash(filepath.Clean(dir)), "/mnt/") {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !isPythonExecutableName(name) {
				continue
			}
			fullPath := filepath.Join(dir, name)
			if !isExecutableFile(fullPath) {
				continue
			}

			collected = append(collected, InterpreterOption{
				Label: name,
				Path:  fullPath,
				Root:  filepath.Dir(filepath.Dir(fullPath)),
				Kind:  InterpreterGlobal,
			})
		}
	}

	sort.Slice(collected, func(i, j int) bool {
		return strings.ToLower(collected[i].Label) < strings.ToLower(collected[j].Label)
	})

	return collected
}

func isPythonExecutableName(name string) bool {
	base := strings.ToLower(name)
	for _, suffix := range []string{".exe", ".cmd", ".bat"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}

	if base == "python" {
		return true
	}
	if !strings.HasPrefix(base, "python") {
		return false
	}
	suffix := strings.TrimPrefix(base, "python")
	if suffix == "" {
		return true
	}
	hasDigit := false
	for _, char := range suffix {
		if char >= '0' && char <= '9' {
			hasDigit = true
			continue
		}
		if char == '.' || (char >= 'a' && char <= 'z') {
			continue
		}
		return false
	}
	return hasDigit
}

func isExecutableFile(fullPath string) bool {
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	if !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		lower := strings.ToLower(fullPath)
		return strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat")
	}
	return info.Mode()&0o111 != 0
}

func isWslEnvironment() bool {
	return os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != ""
}

func isVenvLikeInterpreterPath(path string) bool {
	if path == "" {
		return false
	}

	resolved := canonicalPath(path)
	if resolved == "" {
		resolved = filepath.Clean(path)
	}

	root := interpreterRoot(resolved)
	if root == "" {
		return false
	}

	if fileExists(filepath.Join(root, "pyvenv.cfg")) {
		return true
	}

	if virtualEnv := os.Getenv("VIRTUAL_ENV"); virtualEnv != "" && sameOrWithinPath(resolved, virtualEnv) {
		return true
	}
	if condaPrefix := os.Getenv("CONDA_PREFIX"); condaPrefix != "" && sameOrWithinPath(resolved, condaPrefix) {
		return true
	}

	return false
}

func interpreterRoot(path string) string {
	cleanPath := filepath.Clean(path)
	dir := strings.ToLower(filepath.Base(filepath.Dir(cleanPath)))
	if dir != "bin" && dir != "scripts" {
		return ""
	}
	return filepath.Dir(filepath.Dir(cleanPath))
}

func sameOrWithinPath(path string, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func canonicalPath(path string) string {
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		if absResolved, absErr := filepath.Abs(resolved); absErr == nil {
			return absResolved
		}
		return filepath.Clean(resolved)
	}
	if absPath, err := filepath.Abs(path); err == nil {
		return absPath
	}
	return filepath.Clean(path)
}

func addLocalVenvs(add func(InterpreterOption)) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	entries, err := os.ReadDir(wd)
	if err != nil {
		return
	}

	localOptions := make([]InterpreterOption, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidatePath := filepath.Join(wd, entry.Name())
		if option, ok := interpreterOptionFromRoot(candidatePath, entry.Name(), InterpreterVenv); ok {
			localOptions = append(localOptions, option)
		}
	}

	sort.Slice(localOptions, func(i, j int) bool {
		return localOptions[i].Label < localOptions[j].Label
	})

	for _, option := range localOptions {
		add(option)
	}
}

func addActiveVenv(add func(InterpreterOption)) {
	if virtualEnv := os.Getenv("VIRTUAL_ENV"); virtualEnv != "" {
		name := filepath.Base(virtualEnv)
		if option, ok := interpreterOptionFromRoot(virtualEnv, name, InterpreterVenv); ok {
			add(option)
		}
	}
}

func interpreterOptionFromRoot(root string, name string, kind InterpreterKind) (InterpreterOption, bool) {
	if runtimeInterpreter := filepath.Join(root, "bin", "python"); fileExists(runtimeInterpreter) {
		return InterpreterOption{Label: name + " (venv)", Path: runtimeInterpreter, Root: root, Kind: kind}, true
	}
	if runtimeInterpreter := filepath.Join(root, "Scripts", "python.exe"); fileExists(runtimeInterpreter) {
		return InterpreterOption{Label: name + " (venv)", Path: runtimeInterpreter, Root: root, Kind: kind}, true
	}
	return InterpreterOption{}, false
}

func (option InterpreterOption) activatePath() string {
	root := option.Root
	if root == "" {
		root = filepath.Dir(filepath.Dir(option.Path))
	}
	if root == "" {
		return ""
	}
	if activate := filepath.Join(root, "bin", "activate"); fileExists(activate) {
		return activate
	}
	if activate := filepath.Join(root, "Scripts", "activate"); fileExists(activate) {
		return activate
	}
	return ""
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (option InterpreterOption) pythonVersion() string {
	output, err := exec.Command(option.Path, "-c", "import sys; print(sys.version.split()[0])").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (option InterpreterOption) sizeLabel() string {
	if option.Kind == InterpreterGlobal {
		if info, err := os.Stat(option.Path); err == nil {
			return formatBytes(info.Size())
		}
		return ""
	}

	root := option.Root
	if root == "" {
		root = filepath.Dir(filepath.Dir(option.Path))
	}
	if root == "" {
		return ""
	}

	var sizeBytes int64
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if info, statErr := entry.Info(); statErr == nil {
			sizeBytes += info.Size()
		}
		return nil
	})
	if sizeBytes == 0 {
		return ""
	}
	return formatBytes(sizeBytes)
}

func (option InterpreterOption) installedPackages() []PackageInfo {
	output, err := exec.Command(option.Path, "-m", "pip", "list", "--format=json").CombinedOutput()
	if err != nil {
		return nil
	}

	var payload []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil
	}

	packages := make([]PackageInfo, 0, len(payload))
	for _, item := range payload {
		packages = append(packages, PackageInfo{Name: item.Name, Version: item.Version})
	}
	sort.Slice(packages, func(i, j int) bool {
		return strings.ToLower(packages[i].Name) < strings.ToLower(packages[j].Name)
	})
	return packages
}

func formatBytes(sizeBytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(sizeBytes)
	index := 0
	for value >= 1024 && index < len(units)-1 {
		value /= 1024
		index++
	}
	if index == 0 {
		return fmt.Sprintf("%d %s", sizeBytes, units[index])
	}
	return fmt.Sprintf("%.1f %s", value, units[index])
}
