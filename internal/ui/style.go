package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Icon constants for consistent output.
const (
	IconCheck = "✓"
	IconCross = "✗"
	IconArrow = "→"
)

var (
	normalStyle   = lipgloss.NewStyle()
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	mutedStyle    = lipgloss.NewStyle().Faint(true)
	boldStyle     = lipgloss.NewStyle().Bold(true)
	keywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	valueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	borderPadding = lipgloss.NewStyle().Padding(1, 2)
)

func Header(text string) string {
	return headerStyle.Render(text)
}

func Success(text string) string {
	return successStyle.Render(text)
}

func ErrorMsg(text string) string {
	return errorStyle.Render(text)
}

func Warning(text string) string {
	return warningStyle.Render(text)
}

func Muted(text string) string {
	return mutedStyle.Render(text)
}

func Bold(text string) string {
	return boldStyle.Render(text)
}

func Keyword(text string) string {
	return keywordStyle.Render(text)
}

func Value(text string) string {
	return valueStyle.Render(text)
}

func Box(text string) string {
	return borderPadding.Render(borderStyle.Render(text))
}

// LlamaCppCredit returns the llama.cpp attribution line.
func LlamaCppCredit(version string) string {
	return Muted(fmt.Sprintf("Powered by llama.cpp %s", version))
}
