package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/llemme/internal/tui/styles"
)

// Input wraps a textarea for message input
type Input struct {
	textarea textarea.Model
	width    int
	focused  bool
}

// NewInput creates a new input component
func NewInput() Input {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // No limit
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	// No prompt - the border indicates input area
	ta.Prompt = ""
	ta.Focus()

	return Input{
		textarea: ta,
		focused:  true,
	}
}

// Init returns the initial command
func (i Input) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles input events
func (i Input) Update(msg tea.Msg) (Input, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "shift+enter", "ctrl+j":
			// Insert newline
			i.textarea.InsertRune('\n')
			return i, nil
		case "enter":
			// Plain enter - let parent handle send
			return i, nil
		}
	}

	var cmd tea.Cmd
	i.textarea, cmd = i.textarea.Update(msg)
	return i, cmd
}

// View renders the input
func (i Input) View() string {
	style := styles.InputStyle
	if i.focused {
		style = styles.InputFocusedStyle
	}
	return style.Render(i.textarea.View())
}

// SetWidth sets the input width
func (i *Input) SetWidth(width int) {
	i.width = width
	i.textarea.SetWidth(width - 4) // Account for padding (2 on each side)
}

// SetHeight sets the textarea height
func (i *Input) SetHeight(height int) {
	i.textarea.SetHeight(height)
}

// Focus focuses the input
func (i *Input) Focus() tea.Cmd {
	i.focused = true
	return i.textarea.Focus()
}

// Blur removes focus from the input
func (i *Input) Blur() {
	i.focused = false
	i.textarea.Blur()
}

// Value returns the current input value
func (i Input) Value() string {
	return strings.TrimSpace(i.textarea.Value())
}

// SetValue sets the input value
func (i *Input) SetValue(v string) {
	i.textarea.SetValue(v)
}

// Reset clears the input
func (i *Input) Reset() {
	i.textarea.Reset()
}

// Focused returns whether the input is focused
func (i Input) Focused() bool {
	return i.focused
}
