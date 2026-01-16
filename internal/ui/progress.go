package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

type progressModel struct {
	progress   progress.Model
	total      int64
	downloaded int64
	message    string
	done       bool
}

type progressTickMsg struct{}

func tickProgress() tea.Cmd {
	return tea.Tick(time.Second*100, func(t time.Time) tea.Msg {
		return progressTickMsg{}
	})
}

func initialProgressModel(message string, total int64) progressModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(50),
		progress.WithoutPercentage(),
	)
	return progressModel{
		progress:   p,
		total:      total,
		downloaded: 0,
		message:    message,
		done:       false,
	}
}

func (m progressModel) Init() tea.Cmd {
	return nil
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case progressTickMsg:
		if m.done {
			return m, tea.Quit
		}
		return m, tickProgress()
	}
	return m, nil
}

func (m progressModel) View() string {
	if m.done {
		return m.message + "\n"
	}

	percent := float64(m.downloaded) / float64(m.total)
	width := 50
	filled := int(float64(width) * percent)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	return fmt.Sprintf("\n  %s  %.0f%% │ %s / %s │ %.1f MB/s │ ETA %s\n",
		bar,
		percent*100,
		FormatBytes(m.downloaded),
		FormatBytes(m.total),
		0.0,
		"0s",
	)
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

type ProgressBar struct {
	model   *progressModel
	program *tea.Program
}

func NewProgressBar(message string, total int64) *ProgressBar {
	return &ProgressBar{
		model: &progressModel{},
	}
}

func (p *ProgressBar) Start(message string, total int64) {
	m := initialProgressModel(message, total)
	p.model = &m
	p.program = tea.NewProgram(m)
	go func() {
		p.program.Run()
	}()
}

func (p *ProgressBar) Update(downloaded int64) {
	if p.model != nil {
		p.model.downloaded = downloaded
	}
}

func (p *ProgressBar) Finish(message string) {
	if p.program != nil {
		p.model.done = true
		p.model.message = Success(message)
		p.program.Quit()
	}
}

func (p *ProgressBar) Stop() {
	if p.program != nil {
		p.program.Quit()
	}
}
