package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/llemme/internal/tui/styles"
)

// HeaderStats holds the display statistics for the header
type HeaderStats struct {
	Persona      string
	Model        string
	ContextUsed  int
	ContextMax   int
	TokensPerSec float64
	IsStreaming  bool
}

// Header renders the header bar
type Header struct {
	stats HeaderStats
	width int
}

// NewHeader creates a new header component
func NewHeader() Header {
	return Header{}
}

// SetStats updates the header statistics
func (h *Header) SetStats(stats HeaderStats) {
	h.stats = stats
}

// SetWidth sets the header width
func (h *Header) SetWidth(width int) {
	h.width = width
}

// formatModelName renders the model name with different colors for user/repo:quant
func formatModelName(model string) string {
	if model == "" {
		return ""
	}

	var result string

	// Split user/repo
	if idx := strings.Index(model, "/"); idx != -1 {
		user := model[:idx+1] // include the slash
		rest := model[idx+1:]
		result = styles.HeaderStatStyle.Render(user)

		// Split repo:quant
		if qidx := strings.Index(rest, ":"); qidx != -1 {
			repo := rest[:qidx]
			quant := rest[qidx:] // include the colon
			result += styles.HeaderModelStyle.Render(repo) + styles.HeaderStatStyle.Render(quant)
		} else {
			result += styles.HeaderModelStyle.Render(rest)
		}
	} else {
		// No user prefix, just render as-is
		result = styles.HeaderModelStyle.Render(model)
	}

	return result
}

// View renders the header
func (h Header) View() string {
	if h.width == 0 {
		return ""
	}

	// Build left side: Persona • Model
	var leftPart string
	if h.stats.Persona != "" {
		// When persona is shown, mute the entire model name so persona stands out
		leftPart = styles.HeaderModelStyle.Render(h.stats.Persona) +
			styles.HeaderStatStyle.Render(" • "+h.stats.Model)
	} else {
		leftPart = formatModelName(h.stats.Model)
	}
	modelPart := leftPart

	// Build right-side stats
	var rightParts []string
	divider := styles.HeaderDivider.String()

	// Context usage
	if h.stats.ContextMax > 0 {
		contextStr := fmt.Sprintf("%dk/%dk", h.stats.ContextUsed/1000, h.stats.ContextMax/1000)
		rightParts = append(rightParts, styles.HeaderStatStyle.Render("ctx ")+
			styles.HeaderStatValueStyle.Render(contextStr))
	}

	// Tokens per second
	if h.stats.TokensPerSec > 0 {
		toksStr := fmt.Sprintf("%.1f tok/s", h.stats.TokensPerSec)
		rightParts = append(rightParts, styles.HeaderStatValueStyle.Render(toksStr))
	} else if h.stats.IsStreaming {
		rightParts = append(rightParts, styles.StatusStreamingStyle.Render("streaming..."))
	}

	rightSide := strings.Join(rightParts, " "+divider+" ")

	// Calculate padding to right-align stats
	leftLen := lipgloss.Width(modelPart)
	rightLen := lipgloss.Width(rightSide)
	padding := h.width - leftLen - rightLen - 2 // -2 for left/right padding

	var headerContent string
	if padding > 0 && rightSide != "" {
		headerContent = modelPart + strings.Repeat(" ", padding) + rightSide
	} else {
		headerContent = modelPart
	}

	// Render with full width and divider line
	headerLine := styles.HeaderStyle.Width(h.width).Render(headerContent)
	dividerLine := styles.HorizontalDivider(h.width)

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, dividerLine)
}
