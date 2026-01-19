package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/llemme/internal/tui/styles"
)

// HeaderStats holds the display statistics for the header
type HeaderStats struct {
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

// View renders the header
func (h Header) View() string {
	if h.width == 0 {
		return ""
	}

	// Model name
	modelPart := styles.HeaderModelStyle.Render(h.stats.Model)

	// Context usage
	var contextPart string
	if h.stats.ContextMax > 0 {
		contextStr := fmt.Sprintf("%dk/%dk", h.stats.ContextUsed/1000, h.stats.ContextMax/1000)
		contextPart = styles.HeaderStatStyle.Render("Context: ") +
			styles.HeaderStatValueStyle.Render(contextStr)
	}

	// Tokens per second
	var toksPart string
	if h.stats.TokensPerSec > 0 {
		toksStr := fmt.Sprintf("%.1f tok/s", h.stats.TokensPerSec)
		toksPart = styles.HeaderStatValueStyle.Render(toksStr)
	} else if h.stats.IsStreaming {
		toksPart = styles.StatusStreamingStyle.Render("streaming...")
	}

	// Build header with dividers
	divider := styles.HeaderDivider.String()
	parts := []string{modelPart}

	if contextPart != "" {
		parts = append(parts, contextPart)
	}
	if toksPart != "" {
		parts = append(parts, toksPart)
	}

	content := strings.Join(parts, " "+divider+" ")

	// Render with full width and divider line
	headerLine := styles.HeaderStyle.Width(h.width).Render(content)
	dividerLine := styles.HorizontalDivider(h.width)

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, dividerLine)
}
