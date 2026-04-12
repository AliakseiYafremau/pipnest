//go:build linux || darwin
// +build linux darwin

package manager

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

type pipRunFunc func(ctx context.Context, args ...string) (string, error)

func canonicalPackageName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func parseRequiresFromShow(output string) []string {
	lines := strings.Split(output, "\n")
	requires := make([]string, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Requires:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "Requires:"))
		if raw == "" {
			continue
		}
		for _, item := range strings.Split(raw, ",") {
			name := canonicalPackageName(item)
			if name == "" {
				continue
			}
			requires = append(requires, name)
		}
	}

	return requires
}

func collectTransitiveDependencies(ctx context.Context, pkgName string, runPip pipRunFunc) (map[string]struct{}, error) {
	root := canonicalPackageName(pkgName)
	if root == "" {
		return map[string]struct{}{}, nil
	}

	seen := map[string]struct{}{root: {}}
	deps := map[string]struct{}{}
	queue := []string{root}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		showOut, err := runPip(ctx, "show", current)
		if err != nil {
			continue
		}

		for _, dep := range parseRequiresFromShow(showOut) {
			if dep == "" {
				continue
			}
			if _, exists := seen[dep]; exists {
				continue
			}
			seen[dep] = struct{}{}
			deps[dep] = struct{}{}
			queue = append(queue, dep)
		}
	}

	return deps, nil
}

func listInstalledPackages(ctx context.Context, runPip pipRunFunc) (map[string]struct{}, error) {
	out, err := runPip(ctx, "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	type pkg struct {
		Name string `json:"name"`
	}

	var parsed []pkg
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, err
	}

	installed := make(map[string]struct{}, len(parsed))
	for _, p := range parsed {
		name := canonicalPackageName(p.Name)
		if name == "" {
			continue
		}
		installed[name] = struct{}{}
	}

	return installed, nil
}

func listTopLevelPackages(ctx context.Context, runPip pipRunFunc) (map[string]struct{}, error) {
	out, err := runPip(ctx, "list", "--not-required", "--format", "json")
	if err != nil {
		return nil, err
	}

	type pkg struct {
		Name string `json:"name"`
	}

	var parsed []pkg
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, err
	}

	topLevel := make(map[string]struct{}, len(parsed))
	for _, p := range parsed {
		name := canonicalPackageName(p.Name)
		if name == "" {
			continue
		}
		topLevel[name] = struct{}{}
	}

	return topLevel, nil
}

func removableOrphanDependencies(ctx context.Context, candidateDeps map[string]struct{}, runPip pipRunFunc) ([]string, error) {
	if len(candidateDeps) == 0 {
		return nil, nil
	}

	installed, err := listInstalledPackages(ctx, runPip)
	if err != nil {
		return nil, err
	}

	topLevel, err := listTopLevelPackages(ctx, runPip)
	if err != nil {
		return nil, err
	}

	removable := make([]string, 0, len(candidateDeps))
	for dep := range candidateDeps {
		if dep == "" {
			continue
		}
		if _, ok := installed[dep]; !ok {
			continue
		}
		if _, ok := topLevel[dep]; ok {
			continue
		}
		removable = append(removable, dep)
	}

	sort.Strings(removable)
	return removable, nil
}
