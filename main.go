package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type searchResult struct {
	Name        string
	Version     string
	Description string
	URL         string
}

type searchDoneMsg struct {
	results []searchResult
	err     error
}

type packageNameEntry struct {
	Name string
	URL  string
}

type model struct {
	input    textinput.Model
	width    int
	height   int
	query    string
	results  []searchResult
	selected int
	loading  bool
	err      error
}

const (
	topInputHeight       = 5
	resultLimit          = 25
	resultMouseStartLine = topInputHeight + 5
)

var (
	packageIndexOnce   sync.Once
	packageIndex       []packageNameEntry
	packageIndexErr    error
	packageLinkPattern = regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
)

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Search PyPI packages..."
	ti.Focus()

	return model{input: ti}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
			return m, tea.Quit
		}
		if len(m.results) > 0 {
			switch msg.Type {
			case tea.KeyUp, tea.KeyCtrlP:
				if m.selected > 0 {
					m.selected--
				}
				return m, nil
			case tea.KeyDown, tea.KeyCtrlN:
				if m.selected < len(m.results)-1 {
					m.selected++
				}
				return m, nil
			}
		}
		if msg.Type == tea.KeyEnter {
			query := strings.TrimSpace(m.input.Value())
			if query == "" {
				m.query = ""
				m.results = nil
				m.err = nil
				m.loading = false
				return m, nil
			}

			m.query = query
			m.loading = true
			m.err = nil
			return m, searchPyPI(query)
		}
	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft && len(m.results) > 0 {
			index := msg.Y - resultMouseStartLine
			if index >= 0 && index < len(m.results) {
				m.selected = index
			}
			return m, nil
		}
	case searchDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.results = nil
			m.selected = 0
			return m, nil
		}

		m.err = nil
		m.results = msg.results
		m.selected = 0
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	inputHeight := topInputHeight
	contentHeight := m.height - inputHeight - 1
	if contentHeight < 4 {
		contentHeight = 4
	}
	if contentHeight < 10 {
		contentHeight = 10
	}

	leftPaneWidth := (m.width - 3) / 2
	if leftPaneWidth < 24 {
		leftPaneWidth = 24
	}
	rightPaneWidth := m.width - 3 - leftPaneWidth
	if rightPaneWidth < 24 {
		rightPaneWidth = 24
	}

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(m.width - 2).
		Height(inputHeight - 2)

	leftStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(leftPaneWidth).
		Height(contentHeight - 2)

	rightStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(rightPaneWidth).
		Height(contentHeight - 2)

	status := "Press Enter to search"
	if m.loading {
		status = "Searching..."
	} else if m.query != "" {
		status = fmt.Sprintf("Results for %q", m.query)
	}
	if m.err != nil {
		status = "Search error: " + m.err.Error()
	}

	inputBody := strings.Join([]string{m.input.View(), status}, "\n")
	resultsBody := renderResults(m.results, leftPaneWidth-4, m.selected)
	if resultsBody == "" {
		if m.loading {
			resultsBody = "Loading results..."
		} else {
			resultsBody = "Type a package name and press Enter."
		}
	}
	selectedResult := selectedSearchResult(m.results, m.selected)
	rightBody := renderPackageDetails(selectedResult, rightPaneWidth-4, m.loading, m.query, m.err)

	top := inputStyle.Render(inputBody)
	leftPane := leftStyle.Render(resultsBody)
	rightPane := rightStyle.Render(rightBody)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, lipgloss.NewStyle().Width(1).Render("│"), rightPane)

	return top + "\n" + bottom
}

var stripTagsPattern = regexp.MustCompile(`<[^>]+>`)

func searchPyPI(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := fetchPyPIResults(query)
		return searchDoneMsg{results: results, err: err}
	}
}

func fetchPyPIResults(query string) ([]searchResult, error) {
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

	results := make([]searchResult, 0, limit)
	for i := 0; i < limit; i++ {
		metadata, err := fetchPackageMetadata(scored[i].entry.Name)
		if err != nil {
			results = append(results, searchResult{
				Name:        scored[i].entry.Name,
				URL:         scored[i].entry.URL,
				Description: "Metadata unavailable.",
			})
			continue
		}
		results = append(results, metadata)
	}

	return results, nil
}

type scoredPackage struct {
	entry packageNameEntry
	score int
}

func loadPackageIndex() ([]packageNameEntry, error) {
	packageIndexOnce.Do(func() {
		packageIndex, packageIndexErr = fetchPackageIndex()
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

func fetchPackageMetadata(name string) (searchResult, error) {
	requestURL := "https://pypi.org/pypi/" + url.PathEscape(name) + "/json"
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return searchResult{}, err
	}
	req.Header.Set("User-Agent", "lazypip/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return searchResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return searchResult{}, err
	}

	type packagePayload struct {
		Info struct {
			Version     string            `json:"version"`
			Summary     string            `json:"summary"`
			ProjectURL  string            `json:"package_url"`
			ReleaseURL  string            `json:"release_url"`
			ProjectURLs map[string]string `json:"project_urls"`
		} `json:"info"`
	}

	var payload packagePayload
	if err := jsonUnmarshal(body, &payload); err != nil {
		return searchResult{}, err
	}

	projectURL := payload.Info.ProjectURL
	if projectURL == "" {
		projectURL = payload.Info.ReleaseURL
	}
	if projectURL == "" {
		projectURL = "https://pypi.org/project/" + url.PathEscape(name) + "/"
	}

	return searchResult{
		Name:        name,
		Version:     payload.Info.Version,
		Description: strings.TrimSpace(payload.Info.Summary),
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

func renderResults(results []searchResult, width int, selectedIndex int) string {
	if len(results) == 0 {
		return ""
	}

	if width < 20 {
		width = 20
	}

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("57"))

	headerStyle := lipgloss.NewStyle().Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	var lines []string
	lines = append(lines, headerStyle.Render("Packages"))
	lines = append(lines, subtitleStyle.Render("Arrows or click to highlight"))
	lines = append(lines, "")

	for i, result := range results {
		line := formatResultLine(result, width)
		if i == selectedIndex {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func formatResultLine(result searchResult, width int) string {
	nameStyle := lipgloss.NewStyle().Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	line := nameStyle.Render(result.Name)
	if result.Version != "" {
		line += " " + metaStyle.Render(result.Version)
	}

	if result.Description != "" {
		summaryWidth := width - lipgloss.Width(result.Name) - 3
		if summaryWidth < 18 {
			summaryWidth = 18
		}
		line += metaStyle.Render(" - " + truncateText(strings.TrimSpace(result.Description), summaryWidth))
	}

	if lipgloss.Width(line) > width {
		line = truncateText(line, width)
	}
	return line
}

func selectedSearchResult(results []searchResult, index int) *searchResult {
	if index < 0 || index >= len(results) {
		return nil
	}
	return &results[index]
}

func renderPackageDetails(result *searchResult, width int, loading bool, query string, err error) string {
	if width < 24 {
		width = 24
	}

	titleStyle := lipgloss.NewStyle().Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("230"))

	var lines []string
	lines = append(lines, titleStyle.Render("Package Info"))
	lines = append(lines, metaStyle.Render(strings.TrimSpace(query)))
	lines = append(lines, "")

	if err != nil {
		lines = append(lines, metaStyle.Render("Search error:"))
		lines = append(lines, wrapText(err.Error(), width))
		return strings.Join(lines, "\n")
	}

	if loading && result == nil {
		lines = append(lines, metaStyle.Render("Loading results..."))
		return strings.Join(lines, "\n")
	}

	if result == nil {
		lines = append(lines, metaStyle.Render("Select a package on the left."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, valueStyle.Render(result.Name))
	if result.Version != "" {
		lines = append(lines, metaStyle.Render("Version"))
		lines = append(lines, wrapText(result.Version, width))
	}
	if result.Description != "" {
		lines = append(lines, metaStyle.Render("Summary"))
		lines = append(lines, wrapText(result.Description, width))
	}
	if result.URL != "" {
		lines = append(lines, metaStyle.Render("Project URL"))
		lines = append(lines, wrapText(result.URL, width))
	}

	return strings.Join(lines, "\n")
}

func truncateText(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func wrapText(text string, width int) string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	var line strings.Builder
	for _, word := range words {
		if line.Len() == 0 {
			line.WriteString(word)
			continue
		}
		if line.Len()+1+len(word) > width {
			lines = append(lines, line.String())
			line.Reset()
			line.WriteString(word)
			continue
		}
		line.WriteByte(' ')
		line.WriteString(word)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
