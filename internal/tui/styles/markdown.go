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
	rendererErr       error
	thinkingCache     *glamour.TermRenderer
	thinkingCacheOnce sync.Once
	thinkingErr       error
)

// getRenderer returns a cached glamour renderer.
// Fixed at 80 columns for optimal readability.
func getRenderer() (*glamour.TermRenderer, error) {
	rendererCacheOnce.Do(func() {
		rendererCache, rendererErr = glamour.NewTermRenderer(
			glamour.WithStyles(styles.DarkStyleConfig),
			glamour.WithWordWrap(80),
		)
	})
	return rendererCache, rendererErr
}

// getThinkingRenderer returns a cached glamour renderer with muted colors.
func getThinkingRenderer() (*glamour.TermRenderer, error) {
	thinkingCacheOnce.Do(func() {
		style := styles.DarkStyleConfig
		mutedColor := stringPtr("243") // Muted gray
		style.Document.Color = mutedColor
		style.Paragraph.Color = mutedColor
		style.Text.Color = mutedColor
		style.Emph.Color = mutedColor
		style.Strong.Color = mutedColor

		thinkingCache, thinkingErr = glamour.NewTermRenderer(
			glamour.WithStyles(style),
			glamour.WithWordWrap(80),
		)
	})
	return thinkingCache, thinkingErr
}

func stringPtr(s string) *string {
	return &s
}

// RenderMarkdown renders markdown text for display in the TUI.
// Width constrains output for narrow terminals (glamour renders at 80 cols).
func RenderMarkdown(content string, width int) (string, error) {
	r, err := getRenderer()
	if err != nil {
		return content, err
	}
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
	r, err := getThinkingRenderer()
	if err != nil {
		return content, err
	}
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
