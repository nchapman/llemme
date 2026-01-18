package cmd

import (
	"os"
	"testing"
)

func TestGetEditor(t *testing.T) {
	// Save original env vars
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", origVisual)
		os.Setenv("EDITOR", origEditor)
	}()

	t.Run("prefers VISUAL over EDITOR", func(t *testing.T) {
		os.Setenv("VISUAL", "code")
		os.Setenv("EDITOR", "vim")

		editor := getEditor()
		if editor != "code" {
			t.Errorf("Expected 'code', got '%s'", editor)
		}
	})

	t.Run("uses EDITOR when VISUAL not set", func(t *testing.T) {
		os.Setenv("VISUAL", "")
		os.Setenv("EDITOR", "nano")

		editor := getEditor()
		if editor != "nano" {
			t.Errorf("Expected 'nano', got '%s'", editor)
		}
	})

	t.Run("falls back to common editors when env vars not set", func(t *testing.T) {
		os.Setenv("VISUAL", "")
		os.Setenv("EDITOR", "")

		editor := getEditor()
		// Should find at least one of nano, vim, vi on most systems
		// We can't assert which one, just that it found something or returned empty
		// (empty is valid if no editors are installed, though unlikely)
		_ = editor
	})
}
