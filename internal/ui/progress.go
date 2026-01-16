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
	startTime  time.Time
}

type progressTickMsg struct{}
type progressUpdateMsg struct {
	downloaded int64
}
type progressFinishMsg struct {
	message string
}

func tickProgress() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
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
		startTime:  time.Now(),
	}
}

func (m progressModel) Init() tea.Cmd {
	return tickProgress()
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case progressUpdateMsg:
		m.downloaded = msg.downloaded
		return m, nil
	case progressFinishMsg:
		m.done = true
		m.message = msg.message
		return m, tea.Quit
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

	// Calculate speed and ETA
	elapsed := time.Since(m.startTime).Seconds()
	var speedMBps float64
	var eta string

	if elapsed > 0 && m.downloaded > 0 {
		speedMBps = float64(m.downloaded) / elapsed / (1024 * 1024)
		remaining := m.total - m.downloaded
		if speedMBps > 0 {
			etaSeconds := float64(remaining) / (speedMBps * 1024 * 1024)
			eta = formatETA(etaSeconds)
		} else {
			eta = "calculating..."
		}
	} else {
		eta = "calculating..."
	}

	return fmt.Sprintf("\n  %s  %.0f%% │ %s / %s │ %.1f MB/s │ ETA %s\n",
		bar,
		percent*100,
		FormatBytes(m.downloaded),
		FormatBytes(m.total),
		speedMBps,
		eta,
	)
}

func formatETA(seconds float64) string {
	if seconds < 0 || seconds > 86400*7 {
		return "calculating..."
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if seconds < 3600 {
		mins := int(seconds) / 60
		secs := int(seconds) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(seconds) / 3600
	mins := (int(seconds) % 3600) / 60
	return fmt.Sprintf("%dh %dm", hours, mins)
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
	program *tea.Program
}

func NewProgressBar(message string, total int64) *ProgressBar {
	return &ProgressBar{}
}

func (p *ProgressBar) Start(message string, total int64) {
	m := initialProgressModel(message, total)
	p.program = tea.NewProgram(m)
	go func() {
		p.program.Run()
	}()
}

func (p *ProgressBar) Update(downloaded int64) {
	if p.program != nil {
		p.program.Send(progressUpdateMsg{downloaded: downloaded})
	}
}

func (p *ProgressBar) Finish(message string) {
	if p.program != nil {
		p.program.Send(progressFinishMsg{message: Success(message)})
	}
}

func (p *ProgressBar) Stop() {
	if p.program != nil {
		p.program.Quit()
	}
}
