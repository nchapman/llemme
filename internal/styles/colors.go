// Package styles provides shared color definitions and styling utilities.
package styles

import "github.com/charmbracelet/lipgloss"

// Color palette using AdaptiveColor for light/dark terminal support.
// Dark values match the original CLI color scheme.
var (
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "62", Dark: "12"}   // Blue - headers, user messages
	ColorSecondary = lipgloss.AdaptiveColor{Light: "240", Dark: "250"} // Gray - secondary text
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "34", Dark: "10"}   // Green - success messages
	ColorError     = lipgloss.AdaptiveColor{Light: "160", Dark: "9"}   // Red - error messages
	ColorWarning   = lipgloss.AdaptiveColor{Light: "214", Dark: "11"}  // Yellow - warnings
	ColorAccent    = lipgloss.AdaptiveColor{Light: "99", Dark: "13"}   // Purple - keywords, accents
	ColorMuted     = lipgloss.AdaptiveColor{Light: "246", Dark: "243"} // Dim gray - muted text
	ColorBorder    = lipgloss.AdaptiveColor{Light: "250", Dark: "238"} // Border color
	ColorValue     = lipgloss.AdaptiveColor{Light: "38", Dark: "14"}   // Cyan - values
)

// ANSI color codes as strings for use with glamour/markdown rendering.
const (
	ColorMutedCode = "243"
)

// Icon constants for consistent output.
const (
	IconCheck = "✓"
	IconCross = "✗"
	IconArrow = "→"
)
