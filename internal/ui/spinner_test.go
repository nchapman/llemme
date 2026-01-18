package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func TestInitialSpinModel(t *testing.T) {
	m := initialSpinModel("Loading...")
	if m.message != "Loading..." {
		t.Errorf("Expected message 'Loading...', got '%s'", m.message)
	}
	if m.quitting {
		t.Error("Expected quitting to be false initially")
	}
}

func TestSpinModelView(t *testing.T) {
	t.Run("shows spinner and message when not quitting", func(t *testing.T) {
		m := initialSpinModel("Processing...")
		view := m.View()
		if !strings.Contains(view, "Processing...") {
			t.Errorf("Expected view to contain message, got '%s'", view)
		}
	})

	t.Run("clears line when quitting with empty message", func(t *testing.T) {
		m := spinModel{quitting: true, message: ""}
		view := m.View()
		if view != "\r\033[K" {
			t.Errorf("Expected '\\r\\033[K' for empty quit message, got '%s'", view)
		}
	})

	t.Run("shows message with newline when quitting with message", func(t *testing.T) {
		m := spinModel{quitting: true, message: "Done!"}
		view := m.View()
		if view != "Done!\n" {
			t.Errorf("Expected 'Done!\\n', got '%s'", view)
		}
	})
}

func TestSpinModelUpdate(t *testing.T) {
	t.Run("handles q key", func(t *testing.T) {
		m := initialSpinModel("Test")
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		updated := newModel.(spinModel)
		if !updated.quitting {
			t.Error("Expected quitting to be true after 'q' key")
		}
		if cmd == nil {
			t.Error("Expected quit command")
		}
	})

	t.Run("handles ctrl+c", func(t *testing.T) {
		m := initialSpinModel("Test")
		newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		updated := newModel.(spinModel)
		if !updated.quitting {
			t.Error("Expected quitting to be true after Ctrl+C")
		}
		if cmd == nil {
			t.Error("Expected quit command")
		}
	})

	t.Run("handles finish message with success", func(t *testing.T) {
		m := initialSpinModel("Test")
		newModel, _ := m.Update(spinFinishMsg{success: true, message: "Complete"})
		updated := newModel.(spinModel)
		if !updated.quitting {
			t.Error("Expected quitting to be true")
		}
		// Message should be styled with Success()
		if !strings.Contains(updated.message, "Complete") {
			t.Errorf("Expected message to contain 'Complete', got '%s'", updated.message)
		}
	})

	t.Run("handles finish message with failure", func(t *testing.T) {
		m := initialSpinModel("Test")
		newModel, _ := m.Update(spinFinishMsg{success: false, message: "Failed"})
		updated := newModel.(spinModel)
		if !updated.quitting {
			t.Error("Expected quitting to be true")
		}
		// Message should be styled with ErrorMsg()
		if !strings.Contains(updated.message, "Failed") {
			t.Errorf("Expected message to contain 'Failed', got '%s'", updated.message)
		}
	})

	t.Run("handles spinner tick", func(t *testing.T) {
		m := initialSpinModel("Test")
		_, cmd := m.Update(spinner.TickMsg{})
		// Should return a command for the next tick
		if cmd == nil {
			t.Error("Expected tick command")
		}
	})

	t.Run("returns nil cmd for unknown message", func(t *testing.T) {
		m := initialSpinModel("Test")
		_, cmd := m.Update("unknown")
		if cmd != nil {
			t.Error("Expected nil command for unknown message")
		}
	})
}

func TestSpinModelInit(t *testing.T) {
	m := initialSpinModel("Test")
	cmd := m.Init()
	if cmd == nil {
		t.Error("Expected Init to return a tick command")
	}
}

func TestNewSpinner(t *testing.T) {
	s := NewSpinner()
	if s == nil {
		t.Fatal("Expected non-nil Spinner")
	}
}
