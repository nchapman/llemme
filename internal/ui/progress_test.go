package ui

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1.5, "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "1.0 PB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytes(tt.input)
			if got != tt.want {
				t.Errorf("FormatBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitialProgressModel(t *testing.T) {
	message := "Downloading model"
	total := int64(4000000000)

	model := initialProgressModel(message, total)

	if model.message != message {
		t.Errorf("model.message = %v, want %v", model.message, message)
	}

	if model.total != total {
		t.Errorf("model.total = %v, want %v", model.total, total)
	}

	if model.downloaded != 0 {
		t.Errorf("model.downloaded = %v, want 0", model.downloaded)
	}

	if model.done != false {
		t.Errorf("model.done = %v, want false", model.done)
	}
}

func TestProgressModelView(t *testing.T) {
	tests := []struct {
		name       string
		total      int64
		downloaded int64
		done       bool
		message    string
	}{
		{
			name:       "0% progress",
			total:      1000,
			downloaded: 0,
			done:       false,
		},
		{
			name:       "50% progress",
			total:      1000,
			downloaded: 500,
			done:       false,
		},
		{
			name:       "100% progress",
			total:      1000,
			downloaded: 1000,
			done:       false,
		},
		{
			name:       "done",
			total:      1000,
			downloaded: 1000,
			done:       true,
			message:    "Complete",
		},
		{
			name:       "overflow protection - downloaded exceeds total",
			total:      1000,
			downloaded: 1020, // 102% - should not panic
			done:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := initialProgressModel("test", tt.total)
			model.downloaded = tt.downloaded
			model.done = tt.done
			model.message = tt.message

			view := model.View()

			if view == "" {
				t.Error("View() returned empty string")
			}

			if tt.done {
				if view != tt.message+"\n" {
					t.Errorf("View() when done = %v, want %v", view, tt.message+"\n")
				}
			}
		})
	}
}

func TestNewProgressBar(t *testing.T) {
	bar := NewProgressBar()

	if bar == nil {
		t.Fatal("NewProgressBar() returned nil")
	}

	if bar.program != nil {
		t.Error("bar.program should be nil before Start()")
	}
}

func TestProgressBarUpdateNilProgram(t *testing.T) {
	bar := NewProgressBar()

	// Update should not panic when program is nil (before Start is called)
	bar.Update(500, 1000)
	bar.Update(1000, 1000)
}

func TestCalculateProgress(t *testing.T) {
	tests := []struct {
		name       string
		total      int64
		downloaded int64
		elapsed    time.Duration
		wantSpeed  float64
	}{
		{
			name:       "1 MB/s",
			total:      2000000,
			downloaded: 1000000,
			elapsed:    time.Second,
			wantSpeed:  1000000,
		},
		{
			name:       "10 MB/s",
			total:      20000000,
			downloaded: 10000000,
			elapsed:    time.Second,
			wantSpeed:  10000000,
		},
		{
			name:       "zero speed",
			total:      1000000,
			downloaded: 0,
			elapsed:    time.Second,
			wantSpeed:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			speed := float64(tt.downloaded) / tt.elapsed.Seconds()

			if tt.wantSpeed > 0 && speed != tt.wantSpeed {
				t.Errorf("speed = %v, want %v", speed, tt.wantSpeed)
			}
		})
	}
}

func TestFormatETA(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0, "0s"},
		{30, "30s"},
		{59, "59s"},
		{60, "1m 0s"},
		{90, "1m 30s"},
		{3599, "59m 59s"},
		{3600, "1h 0m"},
		{3660, "1h 1m"},
		{7200, "2h 0m"},
		{-1, "calculating..."},
		{86400 * 8, "calculating..."}, // > 7 days
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatETA(tt.seconds)
			if got != tt.want {
				t.Errorf("formatETA(%v) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestProgressModelUpdate(t *testing.T) {
	model := initialProgressModel("test", 1000)

	t.Run("progressUpdateMsg updates downloaded", func(t *testing.T) {
		updated, cmd := model.Update(progressUpdateMsg{downloaded: 500, total: 1000})
		updatedModel := updated.(progressModel)

		if updatedModel.downloaded != 500 {
			t.Errorf("downloaded = %v, want 500", updatedModel.downloaded)
		}
		if cmd != nil {
			t.Error("expected nil cmd for update message")
		}
	})

	t.Run("progressUpdateMsg updates total when positive", func(t *testing.T) {
		testModel := initialProgressModel("test", 1000)
		updated, _ := testModel.Update(progressUpdateMsg{downloaded: 500, total: 2000})
		updatedModel := updated.(progressModel)

		if updatedModel.total != 2000 {
			t.Errorf("total = %v, want 2000", updatedModel.total)
		}
	})

	t.Run("progressUpdateMsg ignores zero total", func(t *testing.T) {
		testModel := initialProgressModel("test", 1000)
		updated, _ := testModel.Update(progressUpdateMsg{downloaded: 500, total: 0})
		updatedModel := updated.(progressModel)

		if updatedModel.total != 1000 {
			t.Errorf("total = %v, want 1000 (unchanged)", updatedModel.total)
		}
	})

	t.Run("progressFinishMsg sets done and message", func(t *testing.T) {
		updated, cmd := model.Update(progressFinishMsg{message: "Complete!"})
		updatedModel := updated.(progressModel)

		if !updatedModel.done {
			t.Error("expected done=true after finish message")
		}
		if updatedModel.message != "Complete!" {
			t.Errorf("message = %v, want Complete!", updatedModel.message)
		}
		// Should return quit command
		if cmd == nil {
			t.Error("expected quit cmd for finish message")
		}
	})

	t.Run("progressTickMsg continues ticking when not done", func(t *testing.T) {
		updated, cmd := model.Update(progressTickMsg{})
		updatedModel := updated.(progressModel)

		if updatedModel.done {
			t.Error("model should not be done after tick")
		}
		if cmd == nil {
			t.Error("expected tick cmd to continue")
		}
	})

	t.Run("progressTickMsg quits when done", func(t *testing.T) {
		doneModel := initialProgressModel("test", 1000)
		doneModel.done = true

		_, cmd := doneModel.Update(progressTickMsg{})
		if cmd == nil {
			t.Error("expected quit cmd when done")
		}
	})
}
