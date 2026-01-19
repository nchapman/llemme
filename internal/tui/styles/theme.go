package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors - extend existing ui/style.go palette
var (
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "62", Dark: "12"}
	ColorSecondary = lipgloss.AdaptiveColor{Light: "240", Dark: "250"}
	ColorMuted     = lipgloss.AdaptiveColor{Light: "246", Dark: "243"}
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "34", Dark: "10"}
	ColorError     = lipgloss.AdaptiveColor{Light: "160", Dark: "9"}
	ColorWarning   = lipgloss.AdaptiveColor{Light: "214", Dark: "11"}
	ColorAccent    = lipgloss.AdaptiveColor{Light: "99", Dark: "13"}
	ColorBorder    = lipgloss.AdaptiveColor{Light: "250", Dark: "238"}
)

// Header styles
var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 1)

	HeaderDivider = lipgloss.NewStyle().
			Foreground(ColorMuted).
			SetString("│")

	HeaderModelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorAccent)

	HeaderStatStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HeaderStatValueStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)
)

// Message styles
var (
	UserMessageStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true)

	UserPrefixStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			SetString("┃ ")

	AssistantPrefixStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				SetString("  ")

	ThinkingStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	ThinkingPrefixStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true).
				SetString("💭 ")

	ErrorMessageStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Italic(true)
)

// Input styles
var (
	InputStyle = lipgloss.NewStyle().
			Padding(1, 2)

	InputFocusedStyle = lipgloss.NewStyle().
				Padding(1, 2)
)

// Status bar styles
var (
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)

	StatusKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	StatusDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StatusDivider = lipgloss.NewStyle().
			Foreground(ColorMuted).
			SetString(" │ ")

	StatusStreamingStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)
)

// Viewport styles
var (
	ViewportStyle = lipgloss.NewStyle().
		Padding(0, 1)
)

// Border styles
var (
	DividerStyle = lipgloss.NewStyle().
		Foreground(ColorBorder)
)

// HorizontalDivider creates a horizontal line of the given width
func HorizontalDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return DividerStyle.Render(strings.Repeat("─", width))
}
