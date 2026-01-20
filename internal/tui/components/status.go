package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/llemme/internal/tui/styles"
)

// Status represents different status bar states
type StatusState int

const (
	StatusReady StatusState = iota
	StatusStreaming
	StatusError
	StatusHelp
)

// StatusBar renders the footer status bar with keybindings and status
type StatusBar struct {
	state         StatusState
	message       string
	width         int
	scrollPercent float64
}

// NewStatusBar creates a new status bar
func NewStatusBar() StatusBar {
	return StatusBar{
		state: StatusReady,
	}
}

// SetState sets the status bar state
func (s *StatusBar) SetState(state StatusState) {
	s.state = state
	s.message = ""
}

// SetMessage sets a custom status message
func (s *StatusBar) SetMessage(msg string) {
	s.message = msg
}

// SetWidth sets the status bar width
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// SetScrollPercent sets the scroll position percentage (0.0 to 1.0)
func (s *StatusBar) SetScrollPercent(percent float64) {
	s.scrollPercent = percent
}

// View renders the status bar
func (s StatusBar) View() string {
	if s.width == 0 {
		return ""
	}

	dividerLine := styles.HorizontalDivider(s.width)

	var content string
	if s.message != "" {
		content = s.message
	} else {
		switch s.state {
		case StatusStreaming:
			content = s.streamingView()
		case StatusError:
			content = s.errorView()
		case StatusHelp:
			content = s.helpView()
		default:
			content = s.readyView()
		}
	}

	statusLine := styles.StatusBarStyle.Width(s.width).Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, dividerLine, statusLine)
}

func (s StatusBar) readyView() string {
	result := s.keyHint("enter", "send") +
		styles.StatusDivider.String() +
		s.keyHint("tab", "scroll") +
		styles.StatusDivider.String() +
		s.keyHint("/?", "help") +
		styles.StatusDivider.String() +
		s.keyHint("ctrl+c", "quit")

	// Add scroll position indicator
	if s.scrollPercent > 0 && s.scrollPercent < 1 {
		result += styles.StatusDivider.String() +
			styles.StatusDescStyle.Render(fmt.Sprintf("%.0f%%", s.scrollPercent*100))
	}
	return result
}

func (s StatusBar) streamingView() string {
	return styles.StatusStreamingStyle.Render("● Streaming") +
		styles.StatusDivider.String() +
		s.keyHint("esc", "cancel") +
		styles.StatusDivider.String() +
		s.keyHint("ctrl+c", "quit")
}

func (s StatusBar) errorView() string {
	return styles.ErrorMessageStyle.Render("Error occurred") +
		styles.StatusDivider.String() +
		s.keyHint("enter", "retry") +
		styles.StatusDivider.String() +
		s.keyHint("ctrl+c", "quit")
}

func (s StatusBar) helpView() string {
	return s.keyHint("enter", "send") +
		styles.StatusDivider.String() +
		s.keyHint("/help", "commands") +
		styles.StatusDivider.String() +
		s.keyHint("/clear", "clear") +
		styles.StatusDivider.String() +
		s.keyHint("ctrl+c", "quit")
}

func (s StatusBar) keyHint(key, desc string) string {
	return styles.StatusKeyStyle.Render(key) + " " + styles.StatusDescStyle.Render(desc)
}
