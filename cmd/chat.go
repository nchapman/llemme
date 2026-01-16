package cmd

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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

	return chatModel{
		textarea:    ta,
		messages:    []string{},
		viewport:    vp,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		aiStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
		err:         nil,
		api:         api,
		model:       model,
		streamChan:  make(chan string),
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

		if len(m.messages) > 0 {
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(joinMessages(m.messages, m.currentAI)))
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
				m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(joinMessages(m.messages, "")))
				m.textarea.Reset()
				m.viewport.GotoBottom()

				m.generating = true
				return m, tea.Batch(
					m.startGeneration(prompt),
					m.waitForStream(),
				)
			}
		}

	case streamMsg:
		m.currentAI += string(msg)
		m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(joinMessages(m.messages, m.currentAI)))
		m.viewport.GotoBottom()
		return m, m.waitForStream()

	case streamDoneMsg:
		m.generating = false
		finalMsg := m.aiStyle.Render("AI: ") + m.currentAI
		m.messages = append(m.messages, finalMsg)
		m.currentAI = ""
		m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(joinMessages(m.messages, "")))
		m.viewport.GotoBottom()

	case errMsg:
		m.generating = false
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m chatModel) startGeneration(prompt string) tea.Cmd {
	return func() tea.Msg {
		messages := []server.ChatMessage{
			{Role: "user", Content: prompt},
		}

		req := &server.ChatCompletionRequest{
			Model:       m.model,
			Messages:    messages,
			Stream:      true,
			Temperature: m.cfg.Temperature,
			TopP:        m.cfg.TopP,
			MaxTokens:   -1,
		}

		err := m.api.StreamChatCompletion(req, func(content string) {
			m.streamChan <- content
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

func joinMessages(messages []string, currentAI string) string {
	if len(messages) == 0 && currentAI == "" {
		return ""
	}

	result := ""
	for _, msg := range messages {
		result += msg + "\n"
	}

	if currentAI != "" {
		result += lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render("AI: ") + currentAI
	}

	return result
}
