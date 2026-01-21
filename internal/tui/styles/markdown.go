package styles

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

var (
	rendererCache         sync.Map // map[int]*glamour.TermRenderer
	thinkingRendererCache sync.Map // map[int]*glamour.TermRenderer
)

// getRenderer returns a cached renderer for the given width, creating one if needed.
func getRenderer(width int) (*glamour.TermRenderer, error) {
	if cached, ok := rendererCache.Load(width); ok {
		return cached.(*glamour.TermRenderer), nil
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(styles.DarkStyleConfig),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}

	rendererCache.Store(width, r)
	return r, nil
}

// getThinkingRenderer returns a cached thinking renderer for the given width.
func getThinkingRenderer(width int) (*glamour.TermRenderer, error) {
	if cached, ok := thinkingRendererCache.Load(width); ok {
		return cached.(*glamour.TermRenderer), nil
	}

	style := styles.DarkStyleConfig
	mutedColor := stringPtr(ColorMutedCode)
	style.Document.Color = mutedColor
	style.Paragraph.Color = mutedColor
	style.Text.Color = mutedColor
	style.Emph.Color = mutedColor
	style.Strong.Color = mutedColor

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}

	thinkingRendererCache.Store(width, r)
	return r, nil
}

func stringPtr(s string) *string {
	return &s
}

// RenderMarkdown renders markdown text for display in the TUI.
func RenderMarkdown(content string, width int) (string, error) {
	if width <= 0 {
		width = 80
	}
	r, err := getRenderer(width)
	if err != nil {
		return content, err
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content, err
	}
	return strings.TrimRight(rendered, "\n"), nil
}

// RenderThinking renders thinking content with muted glamour styling.
func RenderThinking(content string, width int) (string, error) {
	if width <= 0 {
		width = 80
	}
	r, err := getThinkingRenderer(width)
	if err != nil {
		return content, err
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content, err
	}
	return strings.TrimRight(rendered, "\n"), nil
}
