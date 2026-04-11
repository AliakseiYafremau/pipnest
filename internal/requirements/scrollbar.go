package requirements

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// overlayScrollbarOnBorder replaces the right-border characters of a lipgloss
// rounded-border box with scrollbar thumb/track characters.
// total = total number of items, scroll = first visible item, visibleRows = visible item count.
func overlayScrollbarOnBorder(box string, total, scroll, visibleRows int) string {
	if total <= visibleRows || visibleRows < 1 {
		return box
	}

	thumbHeight := visibleRows * visibleRows / total
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	maxScroll := total - visibleRows
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = scroll * (visibleRows - thumbHeight) / maxScroll
	}

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	lines := strings.Split(box, "\n")
	// Body lines are lines[1] to lines[len-2] (first and last are top/bottom border).
	for i := 1; i < len(lines)-1; i++ {
		bodyIdx := i - 1
		var ch string
		if bodyIdx >= thumbPos && bodyIdx < thumbPos+thumbHeight {
			ch = thumbStyle.Render("█")
		} else {
			ch = barStyle.Render("▐")
		}
		lines[i] = replaceLastBorderChar(lines[i], ch)
	}
	return strings.Join(lines, "\n")
}

// replaceLastBorderChar replaces the last occurrence of the rounded-border
// vertical bar rune (│, U+2502) in a line with the given replacement string.
// The replacement preserves any ANSI reset that follows the original char.
func replaceLastBorderChar(line, replacement string) string {
	// Find the last │ by scanning rune positions from the right.
	idx := strings.LastIndex(line, "│")
	if idx < 0 {
		return line
	}
	return line[:idx] + replacement + line[idx+len("│"):]
}
