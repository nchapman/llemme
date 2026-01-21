package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type spinModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
}

type spinFinishMsg struct {
	success bool
	message string
}

func initialSpinModel(message string) spinModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	return spinModel{
		spinner: s,
		message: message,
	}
}

func (m spinModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case spinFinishMsg:
		m.quitting = true
		if msg.success {
			m.message = Success(msg.message)
		} else {
			m.message = ErrorMsg(msg.message)
		}
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m spinModel) View() string {
	if m.quitting {
		if m.message == "" {
			// Clear the line and stay on it (no newline)
			return "\r\033[K"
		}
		return m.message + "\n"
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

type Spinner struct {
	prog *tea.Program
}

func NewSpinner() *Spinner {
	return &Spinner{}
}

func (s *Spinner) Start(message string) {
	m := initialSpinModel(message)
	s.prog = tea.NewProgram(m)
	go func() {
		s.prog.Run()
	}()
}

func (s *Spinner) Stop(success bool, message string) {
	if s.prog != nil {
		s.prog.Send(spinFinishMsg{success: success, message: message})
		s.prog.Wait()
	}
}

func WithSpinner(message string, fn func() error) error {
	s := NewSpinner()
	s.Start(message)
	err := fn()
	if err != nil {
		s.Stop(false, err.Error())
		return err
	}
	s.Stop(true, message)
	return nil
}
