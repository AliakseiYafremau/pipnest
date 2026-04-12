//go:build linux || darwin
// +build linux darwin

package manager

import (
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type packageNameEntry struct {
	Name string
}

type scoredPackage struct {
	entry packageNameEntry
	score int
}

const dependencySearchLimit = 25

var (
	packageIndexOnce   sync.Once
	packageIndex       []packageNameEntry
	packageIndexErr    error
	packageLinkPattern = regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
)

func SearchPackages(ctx context.Context, query string) ([]Dependency, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	entries, err := loadPackageIndex(ctx)
	if err != nil {
		return nil, err
	}

	queryNorm := normalizeQuery(query)
	scored := make([]scoredPackage, 0, len(entries))
	for _, entry := range entries {
		score := fuzzyScore(queryNorm, normalizeQuery(entry.Name))
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredPackage{entry: entry, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].entry.Name < scored[j].entry.Name
		}
		return scored[i].score > scored[j].score
	})

	limit := dependencySearchLimit
	if len(scored) < limit {
		limit = len(scored)
	}

	deps := make([]Dependency, 0, limit)
	for i := 0; i < limit; i++ {
		deps = append(deps, Dependency{Name: scored[i].entry.Name})
	}

	return deps, nil
}

func loadPackageIndex(ctx context.Context) ([]packageNameEntry, error) {
	packageIndexOnce.Do(func() {
		packageIndex, packageIndexErr = fetchPackageIndex(ctx)
	})

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return packageIndex, packageIndexErr
}

func fetchPackageIndex(ctx context.Context) ([]packageNameEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://pypi.org/simple/", nil)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	matches := packageLinkPattern.FindAllSubmatch(body, -1)
	entries := make([]packageNameEntry, 0, len(matches))
	for _, match := range matches {
		name := html.UnescapeString(strings.TrimSpace(string(match[2])))
		if name == "" {
			continue
		}
		entries = append(entries, packageNameEntry{Name: name})
	}

	return entries, nil
}

func normalizeQuery(text string) string {
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, "-", "")
	text = strings.ReplaceAll(text, "_", "")
	text = strings.ReplaceAll(text, ".", "")
	return text
}

func fuzzyScore(query, candidate string) int {
	if query == "" || candidate == "" {
		return 0
	}
	if candidate == query {
		return 10000 + len(candidate)
	}
	if strings.HasPrefix(candidate, query) {
		return 9000 + len(query)
	}
	if strings.Contains(candidate, query) {
		return 8000 + len(query)
	}

	distance := levenshteinDistance(query, candidate)
	maxLen := len(query)
	if len(candidate) > maxLen {
		maxLen = len(candidate)
	}

	score := maxLen*100 - distance*100
	if score < 0 {
		return 0
	}

	return score
}

func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}

	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}

			deletion := previous[j] + 1
			insertion := current[j-1] + 1
			substitution := previous[j-1] + cost
			current[j] = minInt(deletion, minInt(insertion, substitution))
		}
		previous = current
	}

	return previous[len(b)]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
