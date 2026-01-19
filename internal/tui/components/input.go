package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/llemme/internal/tui/styles"
)

// InputHeightChangedMsg is sent when the input height changes
type InputHeightChangedMsg struct {
	Height int
}

// Input wraps a textarea for message input
type Input struct {
	textarea  textarea.Model
	width     int
	focused   bool
	minHeight int
	maxHeight int
}

// NewInput creates a new input component
func NewInput() Input {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // No limit
	ta.SetHeight(1)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	// No prompt - the border indicates input area
	ta.Prompt = ""
	ta.Focus()

	return Input{
		textarea:  ta,
		focused:   true,
		minHeight: 1,
		maxHeight: 4,
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
			return i, i.checkHeightChange()
		case "enter":
			// Plain enter - let parent handle send
			return i, nil
		}
	}

	var cmd tea.Cmd
	i.textarea, cmd = i.textarea.Update(msg)

	// Check if we need to resize after update
	if heightCmd := i.checkHeightChange(); heightCmd != nil {
		return i, tea.Batch(cmd, heightCmd)
	}
	return i, cmd
}

// checkHeightChange adjusts height based on line count and returns a command if changed
func (i *Input) checkHeightChange() tea.Cmd {
	lines := i.textarea.LineCount()
	targetHeight := max(i.minHeight, min(lines, i.maxHeight))
	currentHeight := i.textarea.Height()

	if currentHeight != targetHeight {
		i.textarea.SetHeight(targetHeight)
		return func() tea.Msg {
			return InputHeightChangedMsg{Height: targetHeight}
		}
	}
	return nil
}

// View renders the input
func (i Input) View() string {
	style := styles.InputStyle
	if i.focused {
		style = styles.InputFocusedStyle
	}
	divider := styles.HorizontalDivider(i.width)
	return lipgloss.JoinVertical(lipgloss.Left, divider, style.Render(i.textarea.View()))
}

// SetWidth sets the input width
func (i *Input) SetWidth(width int) {
	i.width = width
	i.textarea.SetWidth(width - 4) // Account for padding (2 on each side)
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

// Reset clears the input and restores default height
func (i *Input) Reset() {
	i.textarea.Reset()
	i.textarea.SetHeight(i.minHeight)
}

// IsFocused returns whether the input is focused (implements Focusable interface)
func (i Input) IsFocused() bool {
	return i.focused
}

// Focused returns whether the input is focused (alias for IsFocused)
func (i Input) Focused() bool {
	return i.IsFocused()
}

// Height returns the current textarea height
func (i Input) Height() int {
	return i.textarea.Height()
}
