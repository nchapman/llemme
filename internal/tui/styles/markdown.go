package styles

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

// createRenderer creates a new glamour renderer with the given width.
func createRenderer(width int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithStyles(styles.DarkStyleConfig),
		glamour.WithWordWrap(width),
	)
}

// createThinkingRenderer creates a glamour renderer with muted colors.
func createThinkingRenderer(width int) (*glamour.TermRenderer, error) {
	style := styles.DarkStyleConfig
	mutedColor := stringPtr(ColorMutedCode)
	style.Document.Color = mutedColor
	style.Paragraph.Color = mutedColor
	style.Text.Color = mutedColor
	style.Emph.Color = mutedColor
	style.Strong.Color = mutedColor

	return glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
	)
}

func stringPtr(s string) *string {
	return &s
}

// RenderMarkdown renders markdown text for display in the TUI.
func RenderMarkdown(content string, width int) (string, error) {
	if width <= 0 {
		width = 80
	}
	r, err := createRenderer(width)
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
	r, err := createThinkingRenderer(width)
	if err != nil {
		return content, err
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content, err
	}
	return strings.TrimRight(rendered, "\n"), nil
}
