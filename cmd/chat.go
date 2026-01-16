package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/gollama/internal/server"
)

const gap = "\n\n"

type (
	streamMsg     string
	streamDoneMsg struct{}
	errMsg        error
)

type chatModel struct {
	viewport    viewport.Model
	messages    []string
	history     []server.ChatMessage
	textarea    textarea.Model
	senderStyle lipgloss.Style
	aiStyle     lipgloss.Style
	err         error
	generating  bool
	currentAI   string
	api         *server.APIClient
	model       string
	cfg         *configWrapper
	streamChan  chan string
	renderer    *glamour.TermRenderer
}

type configWrapper struct {
	Temperature float64
	TopP        float64
}

func initialChatModel(api *server.APIClient, model string, temperature, topP float64) chatModel {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 4000

	ta.SetWidth(30)
	ta.SetHeight(3)

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(30, 5)
	vp.SetContent(fmt.Sprintf("Chat with %s\nType a message and press Enter to send.\n\n", model))

	ta.KeyMap.InsertNewline.SetEnabled(false)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return chatModel{
		textarea:    ta,
		messages:    []string{},
		history:     []server.ChatMessage{},
		viewport:    vp,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		aiStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
		err:         nil,
		api:         api,
		model:       model,
		streamChan:  make(chan string),
		renderer:    renderer,
		cfg: &configWrapper{
			Temperature: temperature,
			TopP:        topP,
		},
	}
}

func (m chatModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap)

		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(msg.Width-4),
		)

		if len(m.messages) > 0 || m.currentAI != "" {
			m.viewport.SetContent(m.renderMessages())
		}
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.generating {
				return m, tea.Quit
			}
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.generating && m.textarea.Value() != "" {
				prompt := m.textarea.Value()
				userMsg := m.senderStyle.Render("You: ") + prompt
				m.messages = append(m.messages, userMsg)
				m.history = append(m.history, server.ChatMessage{Role: "user", Content: prompt})
				m.viewport.SetContent(m.renderMessages())
				m.textarea.Reset()
				m.viewport.GotoBottom()

				m.streamChan = make(chan string)
				m.generating = true
				return m, tea.Batch(
					m.startGeneration(),
					m.waitForStream(),
				)
			}
		}

	case streamMsg:
		m.currentAI += string(msg)
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, m.waitForStream()

	case streamDoneMsg:
		m.generating = false
		rendered, _ := m.renderer.Render(m.currentAI)
		finalMsg := m.aiStyle.Render("AI:\n") + rendered
		m.messages = append(m.messages, finalMsg)
		m.history = append(m.history, server.ChatMessage{Role: "assistant", Content: m.currentAI})
		m.currentAI = ""
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case errMsg:
		m.generating = false
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m chatModel) startGeneration() tea.Cmd {
	streamChan := m.streamChan
	return func() tea.Msg {
		defer close(streamChan)

		req := &server.ChatCompletionRequest{
			Model:       m.model,
			Messages:    m.history,
			Stream:      true,
			Temperature: m.cfg.Temperature,
			TopP:        m.cfg.TopP,
			MaxTokens:   -1,
		}

		err := m.api.StreamChatCompletion(req, func(content string) {
			streamChan <- content
		})

		if err != nil {
			return errMsg(err)
		}

		return streamDoneMsg{}
	}
}

func (m chatModel) waitForStream() tea.Cmd {
	return func() tea.Msg {
		content, ok := <-m.streamChan
		if !ok {
			return streamDoneMsg{}
		}
		return streamMsg(content)
	}
}

func (m chatModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress Ctrl+C to quit", m.err)
	}

	status := ""
	if m.generating {
		status = m.aiStyle.Render("AI is typing...")
	}

	return fmt.Sprintf(
		"%s\n\n%s%s%s",
		m.viewport.View(),
		status,
		gap,
		m.textarea.View(),
	)
}

func (m chatModel) renderMessages() string {
	if len(m.messages) == 0 && m.currentAI == "" {
		return ""
	}

	var sb strings.Builder
	for _, msg := range m.messages {
		sb.WriteString(msg)
		sb.WriteString("\n\n")
	}

	if m.currentAI != "" {
		rendered, _ := m.renderer.Render(m.currentAI)
		sb.WriteString(m.aiStyle.Render("AI:\n"))
		sb.WriteString(rendered)
	}

	return sb.String()
}
