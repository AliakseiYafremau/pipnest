package pip

import (
	"encoding/json"
	"strings"

	"github.com/Rotlerxd/pipnest/internal/backends"
)

func parseListOutput(out string) ([]backends.Package, error) {
	type listItem struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	var parsed []listItem
	if err := json.Unmarshal([]byte(out), &parsed); err == nil {
		packages := make([]backends.Package, 0, len(parsed))
		for _, p := range parsed {
			packages = append(packages, backends.Package{Name: p.Name, Version: p.Version})
		}
		return packages, nil
	}

	packages := make([]backends.Package, 0)
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
		packages = append(packages, backends.Package{Name: fields[0], Version: fields[1]})
	}

	return packages, nil
}

func parseShowOutput(out string) backends.PackageDetails {
	details := backends.PackageDetails{Raw: strings.TrimSpace(out)}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "name":
			details.Name = value
		case "version":
			details.Version = value
		case "summary":
			details.Summary = value
		case "home-page":
			details.HomePage = value
		case "author":
			details.Author = value
		case "license":
			details.License = value
		}
	}

	return details
}
