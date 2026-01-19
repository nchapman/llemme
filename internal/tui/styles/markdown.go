package styles

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	rendererCache     *glamour.TermRenderer
	rendererCacheOnce sync.Once
)

// getRenderer returns a cached glamour renderer
func getRenderer() *glamour.TermRenderer {
	rendererCacheOnce.Do(func() {
		// Use dark style instead of WithAutoStyle() to avoid terminal color queries
		// that leak escape sequences into stdin during TUI operation
		r, err := glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(80), // Default width, content will be wrapped by lipgloss
		)
		if err == nil {
			rendererCache = r
		}
	})
	return rendererCache
}

// RenderMarkdown renders markdown text for display in the TUI
func RenderMarkdown(content string, width int) (string, error) {
	r := getRenderer()
	if r == nil {
		return content, nil
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content, err
	}
	// Trim trailing newlines from glamour output
	rendered = strings.TrimRight(rendered, "\n")
	return lipgloss.NewStyle().MaxWidth(width).Render(rendered), nil
}

// RenderThinking renders thinking/reasoning content in a muted style
func RenderThinking(content string, width int) string {
	return ThinkingStyle.Width(width).Render(content)
}
