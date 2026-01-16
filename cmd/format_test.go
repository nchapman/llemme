package cmd

import (
	"testing"
	"time"
)

func TestFormatTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Minute), "Just now"},
		{"hours ago", now.Add(-5 * time.Hour), "5h ago"},
		{"days ago", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"weeks ago", now.Add(-15 * 24 * time.Hour), "15d ago"},
		{"old date", now.Add(-60 * 24 * time.Hour), now.Add(-60 * 24 * time.Hour).Format("Jan 2006")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.time)
			if got != tt.want {
				t.Errorf("formatTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"seconds", 45 * time.Second, "45 seconds"},
		{"minutes", 15 * time.Minute, "15 minutes"},
		{"hours", 3 * time.Hour, "3h 0m"},
		{"hours partial", 3*time.Hour + 30*time.Minute, "3h 30m"},
		{"days", 48 * time.Hour, "2.0 days"},
		{"days partial", 36 * time.Hour, "1.5 days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUptime(tt.duration)
			if got != tt.want {
				t.Errorf("formatUptime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{10000, "10.0K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{1000000000, "1.0B"},
		{2500000000, "2.5B"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatNumber(tt.input)
			if got != tt.want {
				t.Errorf("formatNumber(%d) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
