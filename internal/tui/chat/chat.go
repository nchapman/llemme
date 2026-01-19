package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/server"
	"github.com/nchapman/llemme/internal/tui/components"
)

// Message types for communication with the model
type (
	// StreamContentMsg is sent when streaming content arrives
	StreamContentMsg struct {
		Content string
	}

	// StreamThinkingMsg is sent when reasoning content arrives
	StreamThinkingMsg struct {
		Content string
	}

	// StreamDoneMsg indicates streaming is complete
	StreamDoneMsg struct {
		Error   error
		Content string // Full content for history
	}

	// StreamCancelledMsg indicates streaming was cancelled by the user
	StreamCancelledMsg struct{}

	// CommandResultMsg is the result of a slash command
	CommandResultMsg struct {
		Message string
		IsError bool
		Exit    bool
	}
)

// FocusedPane represents which pane has focus
type FocusedPane int

const (
	PaneInput FocusedPane = iota
	PaneMessages
)

// Model is the main TUI chat model
type Model struct {
	// Components
	header   components.Header
	messages components.Messages
	input    components.Input
	status   components.StatusBar

	// API and config
	api     *server.APIClient
	model   string
	cfg     *config.Config
	persona *config.Persona

	// Session state
	chatMessages  []server.ChatMessage
	options       SessionOptions
	pendingReload bool

	// UI state
	width        int
	height       int
	streaming    bool
	quitting     bool
	focusedPane  FocusedPane
	cancelStream context.CancelFunc

	// Key bindings
	keys KeyMap

	// Program reference for sending messages from callbacks
	program *tea.Program
}

// SessionOptions holds runtime-adjustable options for the chat session
type SessionOptions struct {
	// Request-time options (no restart needed)
	Temp          float64
	TopP          float64
	TopK          int
	RepeatPenalty float64
	MinP          float64

	// Server options (require model reload)
	CtxSize   int
	GpuLayers int
	Threads   int

	// Track explicitly set server options (allows setting to 0)
	CtxSizeSet   bool
	GpuLayersSet bool
	ThreadsSet   bool
}

// New creates a new chat TUI model
func New(api *server.APIClient, modelName string, cfg *config.Config, persona *config.Persona) *Model {
	m := &Model{
		header:   components.NewHeader(),
		messages: components.NewMessages(),
		input:    components.NewInput(),
		status:   components.NewStatusBar(),

		api:     api,
		model:   modelName,
		cfg:     cfg,
		persona: persona,

		chatMessages: []server.ChatMessage{},
		keys:         DefaultKeyMap(),
	}

	// Initialize system prompt
	m.initSystemPrompt()

	// Set initial header stats
	m.header.SetStats(components.HeaderStats{
		Model: modelName,
	})

	return m
}

// SetProgram sets the tea.Program reference for sending messages
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// SetInitialServerOptions sets the initial server options from CLI flags
func (m *Model) SetInitialServerOptions(ctxSize, gpuLayers, threads int, ctxSizeSet, gpuLayersSet, threadsSet bool) {
	m.options.CtxSize = ctxSize
	m.options.GpuLayers = gpuLayers
	m.options.Threads = threads
	m.options.CtxSizeSet = ctxSizeSet
	m.options.GpuLayersSet = gpuLayersSet
	m.options.ThreadsSet = threadsSet
}

// SetSamplingOptions sets the sampling options from CLI flags
func (m *Model) SetSamplingOptions(temp, topP, minP, repeatPenalty float64, topK int) {
	if temp != 0 {
		m.options.Temp = temp
	}
	if topP != 0 {
		m.options.TopP = topP
	}
	if topK != 0 {
		m.options.TopK = topK
	}
	if minP != 0 {
		m.options.MinP = minP
	}
	if repeatPenalty != 0 {
		m.options.RepeatPenalty = repeatPenalty
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return m.input.Init()
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.KeyMsg:
		// Handle global keys first
		switch {
		case msg.Type == tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case msg.Type == tea.KeyEsc:
			if m.streaming {
				// Cancel streaming - this cancels the HTTP request
				if m.cancelStream != nil {
					m.cancelStream()
				}
				m.messages.CancelStreaming()
				m.stopStreaming()
				return m, nil
			}
			// Esc returns focus to input
			if m.focusedPane == PaneMessages {
				m.focusedPane = PaneInput
				return m, m.input.Focus()
			}

		case msg.Type == tea.KeyTab:
			// Toggle focus between input and messages
			return m, m.toggleFocus()

		case msg.Type == tea.KeyEnter && m.focusedPane == PaneInput && !m.streaming:
			// Send message (only when input is focused)
			value := m.input.Value()
			if value != "" {
				m.input.Reset()

				// Check for slash commands
				if strings.HasPrefix(value, "/") {
					return m, m.handleCommand(value)
				}

				// Send user message
				return m, m.sendMessage(value)
			}
		}

		// Route key events to focused pane
		switch m.focusedPane {
		case PaneMessages:
			var cmd tea.Cmd
			m.messages, cmd = m.messages.Update(msg)
			cmds = append(cmds, cmd)
		case PaneInput:
			if !m.streaming {
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case StreamContentMsg:
		m.messages.AppendStreamContent(msg.Content)

	case StreamThinkingMsg:
		m.messages.AppendStreamThinking(msg.Content)

	case StreamDoneMsg:
		m.messages.FinishStreaming()
		m.stopStreaming()
		if msg.Error != nil {
			m.messages.AddMessage(components.Message{
				Role:    components.RoleError,
				Content: msg.Error.Error(),
			})
		} else if msg.Content != "" {
			// Add to chat history
			m.chatMessages = append(m.chatMessages, server.ChatMessage{
				Role:    "assistant",
				Content: msg.Content,
			})
		}
		cmds = append(cmds, m.input.Focus())

	case StreamCancelledMsg:
		// Stream was cancelled by user - just clean up, no error message
		m.stopStreaming()
		cmds = append(cmds, m.input.Focus())

	case CommandResultMsg:
		if msg.Exit {
			m.quitting = true
			return m, tea.Quit
		}
		if msg.Message != "" {
			role := components.RoleSystem
			if msg.IsError {
				role = components.RoleError
			}
			m.messages.AddMessage(components.Message{
				Role:    role,
				Content: msg.Message,
			})
		}

	case spinner.TickMsg:
		if m.streaming {
			var cmd tea.Cmd
			m.messages, cmd = m.messages.Update(msg)
			cmds = append(cmds, cmd)
		}

	case components.InputHeightChangedMsg:
		// Input height changed, recalculate layout
		m.updateLayout()

	default:
		// Update input for other messages (like blink)
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the model
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Update scroll percentage for status bar
	m.status.SetScrollPercent(m.messages.ScrollPercent())

	var sections []string

	// Header
	sections = append(sections, m.header.View())

	// Messages viewport
	sections = append(sections, m.messages.View())

	// Input
	sections = append(sections, m.input.View())

	// Status bar
	sections = append(sections, m.status.View())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Layout constants
const (
	headerHeight  = 2 // content + divider
	statusHeight  = 2 // divider + content
	inputOverhead = 2 // blank line + divider
)

// updateLayout recalculates component sizes
func (m *Model) updateLayout() {
	// Input height is dynamic based on content
	inputHeight := m.input.Height()
	editorHeight := inputOverhead + inputHeight

	// Messages viewport gets remaining space
	messagesHeight := max(1, m.height-headerHeight-statusHeight-editorHeight)

	m.header.SetWidth(m.width)
	m.messages.SetSize(m.width, messagesHeight)
	m.input.SetWidth(m.width)
	m.status.SetWidth(m.width)
}

// toggleFocus switches focus between input and messages panes
func (m *Model) toggleFocus() tea.Cmd {
	switch m.focusedPane {
	case PaneInput:
		m.focusedPane = PaneMessages
		m.input.Blur()
		return nil
	case PaneMessages:
		m.focusedPane = PaneInput
		return m.input.Focus()
	}
	return nil
}

// initSystemPrompt sets up the initial system message
func (m *Model) initSystemPrompt() {
	sysPrompt := ""
	if m.persona != nil && m.persona.System != "" {
		sysPrompt = m.persona.System
	}
	if sysPrompt == "" {
		sysPrompt = "You are a helpful assistant."
	}
	m.chatMessages = []server.ChatMessage{{Role: "system", Content: sysPrompt}}
}

// sendMessage sends a user message and starts streaming
func (m *Model) sendMessage(content string) tea.Cmd {
	// Ensure program is initialized for streaming callbacks
	if m.program == nil {
		return func() tea.Msg {
			return StreamDoneMsg{Error: fmt.Errorf("internal error: program not initialized")}
		}
	}

	// Add to UI
	m.messages.AddMessage(components.Message{
		Role:    components.RoleUser,
		Content: content,
	})

	// Add to chat history
	m.chatMessages = append(m.chatMessages, server.ChatMessage{
		Role:    "user",
		Content: content,
	})

	// Start streaming and get spinner tick command
	spinnerCmd := m.startStreaming()

	// Create cancellable context for this stream
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel

	// Capture values for the goroutine
	api := m.api
	model := m.model
	messages := make([]server.ChatMessage, len(m.chatMessages))
	copy(messages, m.chatMessages)
	program := m.program

	// Build request
	req := &server.ChatCompletionRequest{
		Model:           model,
		Messages:        messages,
		Stream:          true,
		ReasoningFormat: "auto",
	}
	req.Temperature = m.resolveFloat(m.options.Temp, "temp")
	req.TopP = m.resolveFloat(m.options.TopP, "top-p")
	req.TopK = m.resolveInt(m.options.TopK, "top-k")
	req.MinP = m.resolveFloat(m.options.MinP, "min-p")
	req.RepeatPenalty = m.resolveFloat(m.options.RepeatPenalty, "repeat-penalty")

	streamCmd := func() tea.Msg {
		var fullContent strings.Builder

		cb := server.StreamCallback{
			ContentCallback: func(content string) {
				fullContent.WriteString(content)
				if program != nil {
					program.Send(StreamContentMsg{Content: content})
				}
			},
			ReasoningCallback: func(reasoning string) {
				if program != nil {
					program.Send(StreamThinkingMsg{Content: reasoning})
				}
			},
		}

		err := api.StreamChatCompletion(ctx, req, cb)

		// Handle cancellation distinctly - no error shown to user
		if errors.Is(err, context.Canceled) {
			return StreamCancelledMsg{}
		}

		return StreamDoneMsg{Error: err, Content: fullContent.String()}
	}

	return tea.Batch(spinnerCmd, streamCmd)
}

// resolveFloat returns the first non-zero value from: session, persona, config
func (m *Model) resolveFloat(sessionVal float64, key string) float64 {
	if sessionVal != 0 {
		return sessionVal
	}
	if m.persona != nil {
		if v := m.persona.GetFloatOption(key, 0); v != 0 {
			return v
		}
	}
	return m.cfg.LlamaCpp.GetFloatOption(key, 0)
}

// resolveInt returns the first non-zero value from: session, persona, config
func (m *Model) resolveInt(sessionVal int, key string) int {
	if sessionVal != 0 {
		return sessionVal
	}
	return m.getConfigInt(key)
}

func (m *Model) getConfigInt(key string) int {
	if m.persona != nil {
		if val, ok := m.persona.Options[key]; ok {
			switch v := val.(type) {
			case int:
				return v
			case float64:
				return int(v)
			}
		}
	}
	return m.cfg.LlamaCpp.GetIntOption(key, 0)
}

// startStreaming sets streaming state consistently and returns spinner tick command
func (m *Model) startStreaming() tea.Cmd {
	m.streaming = true
	m.status.SetState(components.StatusStreaming)
	return m.messages.StartStreaming()
}

// stopStreaming clears streaming state consistently
func (m *Model) stopStreaming() {
	m.streaming = false
	m.status.SetState(components.StatusReady)
	m.cancelStream = nil
}
