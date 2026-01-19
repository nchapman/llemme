package styles

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

var (
	rendererCache     *glamour.TermRenderer
	rendererCacheOnce sync.Once
	thinkingCache     *glamour.TermRenderer
	thinkingCacheOnce sync.Once
)

// getRenderer returns a cached glamour renderer.
// Fixed at 80 columns for optimal readability.
func getRenderer() *glamour.TermRenderer {
	rendererCacheOnce.Do(func() {
		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(styles.DarkStyleConfig),
			glamour.WithWordWrap(80),
		)
		if err == nil {
			rendererCache = r
		}
	})
	return rendererCache
}

// getThinkingRenderer returns a cached glamour renderer with muted colors.
func getThinkingRenderer() *glamour.TermRenderer {
	thinkingCacheOnce.Do(func() {
		style := styles.DarkStyleConfig
		mutedColor := stringPtr("243") // Muted gray
		style.Document.Color = mutedColor
		style.Paragraph.Color = mutedColor
		style.Text.Color = mutedColor
		style.Emph.Color = mutedColor
		style.Strong.Color = mutedColor

		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(style),
			glamour.WithWordWrap(80),
		)
		if err == nil {
			thinkingCache = r
		}
	})
	return thinkingCache
}

func stringPtr(s string) *string {
	return &s
}

// RenderMarkdown renders markdown text for display in the TUI.
// Width constrains output for narrow terminals (glamour renders at 80 cols).
func RenderMarkdown(content string, width int) (string, error) {
	r := getRenderer()
	if r == nil {
		return content, nil
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content, err
	}
	rendered = strings.TrimRight(rendered, "\n")
	return lipgloss.NewStyle().MaxWidth(width).Render(rendered), nil
}

// RenderThinking renders thinking content with muted glamour styling.
func RenderThinking(content string, width int) (string, error) {
	r := getThinkingRenderer()
	if r == nil {
		return content, nil
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content, err
	}
	rendered = strings.TrimRight(rendered, "\n")
	return lipgloss.NewStyle().MaxWidth(width).Render(rendered), nil
}
