package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/lleme/internal/styles"
)

// Re-export colors from shared styles package for convenience.
var (
	ColorPrimary   = styles.ColorPrimary
	ColorSecondary = styles.ColorSecondary
	ColorMuted     = styles.ColorMuted
	ColorSuccess   = styles.ColorSuccess
	ColorError     = styles.ColorError
	ColorWarning   = styles.ColorWarning
	ColorAccent    = styles.ColorAccent
	ColorBorder    = styles.ColorBorder
	ColorValue     = styles.ColorValue
)

// Re-export color codes for glamour markdown styling.
const ColorMutedCode = styles.ColorMutedCode

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

	ErrorMessageStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Italic(true)
)

// Input styles
var (
	InputStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2).
			Foreground(ColorMuted)

	InputFocusedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				PaddingRight(2)
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
