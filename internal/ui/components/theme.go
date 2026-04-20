package components

import "github.com/charmbracelet/lipgloss"

// Theme centralizes shared UI tokens for reusable components.
type Theme struct {
	Gap                    int
	LeftTopRatio           float64
	RightBottomHeight      int
	PanelPaddingHorizontal int
	PanelPaddingVertical   int
	BorderColor            lipgloss.Color
	TitleColor             lipgloss.Color
	SelectedColor          lipgloss.Color
	MutedTextColor         lipgloss.Color
}

var UI = Theme{
	Gap:                    1,
	LeftTopRatio:           0.68,
	RightBottomHeight:      5,
	PanelPaddingHorizontal: 1,
	PanelPaddingVertical:   0,
	BorderColor:            lipgloss.Color("240"),
	TitleColor:             lipgloss.Color("252"),
	SelectedColor:          lipgloss.Color("81"),
	MutedTextColor:         lipgloss.Color("244"),
}
