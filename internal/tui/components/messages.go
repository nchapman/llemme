package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nchapman/llemme/internal/tui/styles"
)

// MessageRole represents who sent the message
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleError     MessageRole = "error"
)

// Message represents a chat message
type Message struct {
	Role     MessageRole
	Content  string
	Thinking string // Reasoning/thinking content (shown muted)
	rendered string // Cached rendered content
}

// Messages manages the scrollable message viewport
type Messages struct {
	viewport viewport.Model
	messages []Message
	width    int
	height   int

	// Streaming state
	streaming         bool
	streamingContent  string
	streamingThinking string
}

// NewMessages creates a new messages viewport
func NewMessages() Messages {
	vp := viewport.New(0, 0) // Size set via SetSize()
	vp.Style = styles.ViewportStyle
	return Messages{
		viewport: vp,
		messages: []Message{},
	}
}

// Init returns the initial command
func (m Messages) Init() tea.Cmd {
	return nil
}

// Update handles viewport events
func (m Messages) Update(msg tea.Msg) (Messages, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle scroll keys explicitly
		switch {
		case key.Matches(msg, m.viewport.KeyMap.Up):
			m.viewport.ScrollUp(1)
			return m, nil
		case key.Matches(msg, m.viewport.KeyMap.Down):
			m.viewport.ScrollDown(1)
			return m, nil
		case key.Matches(msg, m.viewport.KeyMap.PageUp):
			m.viewport.PageUp()
			return m, nil
		case key.Matches(msg, m.viewport.KeyMap.PageDown):
			m.viewport.PageDown()
			return m, nil
		case key.Matches(msg, m.viewport.KeyMap.HalfPageUp):
			m.viewport.HalfPageUp()
			return m, nil
		case key.Matches(msg, m.viewport.KeyMap.HalfPageDown):
			m.viewport.HalfPageDown()
			return m, nil
		}
		// Handle home/end keys for go to top/bottom
		switch msg.String() {
		case "home", "g":
			m.viewport.GotoTop()
			return m, nil
		case "end", "G":
			m.viewport.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the messages viewport
func (m Messages) View() string {
	return m.viewport.View()
}

// SetSize sets the viewport dimensions
func (m *Messages) SetSize(width, height int) {
	// Clear render cache when width changes
	if m.width != width {
		for i := range m.messages {
			m.messages[i].rendered = ""
		}
	}
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height
	m.refresh()
}

// GetSize returns the current viewport dimensions (implements Sizeable interface)
func (m Messages) GetSize() (width, height int) {
	return m.width, m.height
}

// AddMessage adds a message to the list
func (m *Messages) AddMessage(msg Message) {
	m.messages = append(m.messages, msg)
	m.refresh()
	m.viewport.GotoBottom()
}

// ClearMessages removes all messages
func (m *Messages) ClearMessages() {
	m.messages = []Message{}
	m.refresh()
}

// StartStreaming begins a streaming response
func (m *Messages) StartStreaming() {
	m.streaming = true
	m.streamingContent = ""
	m.streamingThinking = ""
	m.refresh()
}

// AppendStreamContent adds content to the current streaming message
func (m *Messages) AppendStreamContent(content string) {
	m.streamingContent += content
	m.refresh()
	m.viewport.GotoBottom()
}

// AppendStreamThinking adds thinking content to the current streaming message
func (m *Messages) AppendStreamThinking(thinking string) {
	m.streamingThinking += thinking
	m.refresh()
	m.viewport.GotoBottom()
}

// FinishStreaming completes the streaming message
func (m *Messages) FinishStreaming() {
	if m.streaming {
		m.messages = append(m.messages, Message{
			Role:     RoleAssistant,
			Content:  m.streamingContent,
			Thinking: m.streamingThinking,
		})
		m.streaming = false
		m.streamingContent = ""
		m.streamingThinking = ""
		m.refresh()
		m.viewport.GotoBottom()
	}
}

// CancelStreaming cancels the current streaming without adding message
func (m *Messages) CancelStreaming() {
	m.streaming = false
	m.streamingContent = ""
	m.streamingThinking = ""
	m.refresh()
}

// IsStreaming returns whether currently streaming
func (m Messages) IsStreaming() bool {
	return m.streaming
}

// Messages returns the message list
func (m Messages) MessagesList() []Message {
	return m.messages
}

// refresh rebuilds the viewport content
func (m *Messages) refresh() {
	contentWidth := m.width - 4 // Account for viewport padding

	var sb strings.Builder

	for i := range m.messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		// Use cached render if available
		if m.messages[i].rendered == "" {
			m.messages[i].rendered = m.renderMessage(m.messages[i], contentWidth)
		}
		sb.WriteString(m.messages[i].rendered)
	}

	// Render streaming content if active
	if m.streaming {
		if len(m.messages) > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(m.renderStreaming(contentWidth))
	}

	m.viewport.SetContent(sb.String())
}

func (m Messages) renderMessage(msg Message, width int) string {
	var sb strings.Builder

	switch msg.Role {
	case RoleUser:
		prefix := styles.UserPrefixStyle.String()
		content := styles.UserMessageStyle.Render(msg.Content)
		// Indent each line with the prefix
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(prefix + line)
		}

	case RoleAssistant:
		// Render thinking first if present
		if msg.Thinking != "" {
			rendered, err := styles.RenderThinking(msg.Thinking, width)
			if err != nil {
				rendered = msg.Thinking
			}
			sb.WriteString(strings.TrimSpace(rendered))
			sb.WriteString("\n\n")
		}

		// Render content with markdown (glamour handles margin)
		rendered, err := styles.RenderMarkdown(msg.Content, width)
		if err != nil {
			rendered = msg.Content
		}
		sb.WriteString(strings.TrimSpace(rendered))

	case RoleSystem:
		content := styles.SystemMessageStyle.Width(width).Render(msg.Content)
		sb.WriteString(content)

	case RoleError:
		content := styles.ErrorMessageStyle.Width(width).Render("Error: " + msg.Content)
		sb.WriteString(content)
	}

	return sb.String()
}

func (m Messages) renderStreaming(width int) string {
	var sb strings.Builder

	// Show thinking if present
	if m.streamingThinking != "" {
		rendered, err := styles.RenderThinking(m.streamingThinking, width)
		if err != nil {
			rendered = m.streamingThinking
		}
		sb.WriteString(strings.TrimSpace(rendered))
		sb.WriteString("\n\n")
	}

	// Show content with streaming indicator
	if m.streamingContent == "" {
		sb.WriteString(styles.StatusStreamingStyle.MarginLeft(2).Render("..."))
	} else {
		// Render markdown for streaming content (glamour handles margin)
		rendered, err := styles.RenderMarkdown(m.streamingContent, width)
		if err != nil {
			rendered = m.streamingContent
		}
		rendered = strings.TrimSpace(rendered) + styles.StatusStreamingStyle.Render("▌")
		sb.WriteString(rendered)
	}

	return sb.String()
}

// ScrollPercent returns the scroll percentage
func (m Messages) ScrollPercent() float64 {
	return m.viewport.ScrollPercent()
}
