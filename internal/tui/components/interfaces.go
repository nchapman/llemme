package components

import tea "github.com/charmbracelet/bubbletea"

// Focusable components can receive/lose keyboard focus
type Focusable interface {
	Focus() tea.Cmd
	Blur()
	IsFocused() bool
}

// Sizeable components can be resized (2D)
type Sizeable interface {
	SetSize(width, height int)
	GetSize() (width, height int)
}

// WidthSizeable components only need width (1D)
type WidthSizeable interface {
	SetWidth(width int)
}
