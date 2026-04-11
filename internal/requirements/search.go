package requirements

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Result struct {
	Name        string
	Version     string
	Description string
	Readme      string
	URL         string
}

type DoneMsg struct {
	Results []Result
	Err     error
}

type DescriptionLoadedMsg struct {
	Index  int
	Result Result
}

type packageNameEntry struct {
	Name string
	URL  string
}

type scoredPackage struct {
	entry packageNameEntry
	score int
}

const resultLimit = 25

var (
	packageIndexOnce   sync.Once
	packageIndex       []packageNameEntry
	packageIndexErr    error
	packageLinkPattern = regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	indexCache         *IndexCache
)

func Search(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := fetchResults(query)
		return DoneMsg{Results: results, Err: err}
	}
}

func FetchDescription(index int, name string) tea.Cmd {
	return func() tea.Msg {
		result, err := fetchPackageMetadata(name)
		if err != nil {
			return DescriptionLoadedMsg{Index: index, Result: Result{Name: name, Description: "Metadata unavailable."}}
		}
		return DescriptionLoadedMsg{Index: index, Result: result}
	}
}

func fetchResults(query string) ([]Result, error) {
	entries, err := loadPackageIndex()
	if err != nil {
		return nil, err
	}

	scored := make([]scoredPackage, 0, len(entries))
	queryNorm := normalizeQuery(query)
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

	limit := resultLimit
	if len(scored) < limit {
		limit = len(scored)
	}

	results := make([]Result, 0, limit)
	for i := 0; i < limit; i++ {
		results = append(results, Result{
			Name: scored[i].entry.Name,
			URL:  scored[i].entry.URL,
		})
	}

	return results, nil
}

func loadPackageIndex() ([]packageNameEntry, error) {
	packageIndexOnce.Do(func() {
		indexCache = NewIndexCache()
		packageIndex, packageIndexErr = indexCache.LoadOrFetch()
	})
	return packageIndex, packageIndexErr
}

func fetchPackageIndex() ([]packageNameEntry, error) {
	req, err := http.NewRequest(http.MethodGet, "https://pypi.org/simple/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lazypip/1.0")

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
		entries = append(entries, packageNameEntry{
			Name: name,
			URL:  "https://pypi.org" + string(match[1]),
		})
	}

	return entries, nil
}

func fetchPackageMetadata(name string) (Result, error) {
	requestURL := "https://pypi.org/pypi/" + url.PathEscape(name) + "/json"
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", "lazypip/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, err
	}

	type packagePayload struct {
		Info struct {
			Version     string            `json:"version"`
			Summary     string            `json:"summary"`
			Description string            `json:"description"`
			ProjectURL  string            `json:"package_url"`
			ReleaseURL  string            `json:"release_url"`
			ProjectURLs map[string]string `json:"project_urls"`
		} `json:"info"`
	}

	var payload packagePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return Result{}, err
	}

	projectURL := payload.Info.ProjectURL
	if projectURL == "" {
		projectURL = payload.Info.ReleaseURL
	}
	if projectURL == "" {
		projectURL = "https://pypi.org/project/" + url.PathEscape(name) + "/"
	}

	readme := strings.TrimSpace(payload.Info.Description)

	return Result{
		Name:        name,
		Version:     payload.Info.Version,
		Description: strings.TrimSpace(payload.Info.Summary),
		Readme:      readme,
		URL:         projectURL,
	}, nil
}

func jsonUnmarshal(body []byte, target any) error {
	return json.Unmarshal(body, target)
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
		return 10_000 + len(candidate)
	}
	if strings.Contains(candidate, query) {
		return 8_000 + len(query)
	}
	if strings.HasPrefix(candidate, query) {
		return 9_000 + len(query)
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
			current[j] = min(deletion, min(insertion, substitution))
		}
		previous = current
	}

	return previous[len(b)]
}
