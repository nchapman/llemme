package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nchapman/llemme/internal/tui/styles"
)

// Completion represents a single completion item
type Completion struct {
	Text        string // Display text (e.g., "/help")
	Description string // Description shown beside it
	Value       string // Value to insert
}

// Completions is a popup component for showing completion options
type Completions struct {
	items    []Completion // All items
	filtered []Completion // Filtered items
	selected int          // Selected index in filtered list
	query    string       // Current filter query
	open     bool         // Whether popup is visible
	maxItems int          // Max items to show
}

// NewCompletions creates a new completions component
func NewCompletions() *Completions {
	return &Completions{
		maxItems: 10,
	}
}

// Open opens the completions popup with the given items
func (c *Completions) Open(items []Completion) {
	c.items = items
	c.filtered = items
	c.selected = 0
	c.query = ""
	c.open = true
}

// Close closes the completions popup
func (c *Completions) Close() {
	c.open = false
	c.items = nil
	c.filtered = nil
	c.selected = 0
	c.query = ""
}

// IsOpen returns whether the popup is open
func (c *Completions) IsOpen() bool {
	return c.open
}

// Filter filters the items by query and updates the filtered list
func (c *Completions) Filter(query string) {
	// Skip if query hasn't changed
	if c.query == query {
		return
	}
	c.query = query

	if query == "" {
		c.filtered = c.items
		c.selected = 0
		return
	}

	query = strings.ToLower(query)
	c.filtered = nil
	for _, item := range c.items {
		if strings.HasPrefix(strings.ToLower(item.Text), query) ||
			strings.HasPrefix(strings.ToLower(item.Value), query) {
			c.filtered = append(c.filtered, item)
		}
	}

	// Reset selection if out of bounds
	if c.selected >= len(c.filtered) {
		c.selected = max(0, len(c.filtered)-1)
	}

	// Close if no matches
	if len(c.filtered) == 0 {
		c.Close()
	}
}

// MoveUp moves selection up
func (c *Completions) MoveUp() {
	if len(c.filtered) == 0 {
		return
	}
	c.selected--
	if c.selected < 0 {
		c.selected = len(c.filtered) - 1
	}
}

// MoveDown moves selection down
func (c *Completions) MoveDown() {
	if len(c.filtered) == 0 {
		return
	}
	c.selected++
	if c.selected >= len(c.filtered) {
		c.selected = 0
	}
}

// Selected returns the currently selected completion, or nil if none
func (c *Completions) Selected() *Completion {
	if len(c.filtered) == 0 || c.selected < 0 || c.selected >= len(c.filtered) {
		return nil
	}
	return &c.filtered[c.selected]
}

// Count returns the number of filtered items
func (c *Completions) Count() int {
	return len(c.filtered)
}

// View renders the completions popup
func (c *Completions) View() string {
	if !c.open || len(c.filtered) == 0 {
		return ""
	}

	// Determine visible items (handle scrolling)
	start := 0
	end := len(c.filtered)
	if end > c.maxItems {
		// Center selection in view when possible
		start = c.selected - c.maxItems/2
		if start < 0 {
			start = 0
		}
		end = start + c.maxItems
		if end > len(c.filtered) {
			end = len(c.filtered)
			start = end - c.maxItems
		}
	}

	// Build rows
	var rows []string
	for i := start; i < end; i++ {
		item := c.filtered[i]
		row := c.renderItem(item, i == c.selected)
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")

	// Style the popup with a border
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(0, 1)

	return popupStyle.Render(content)
}

// renderItem renders a single completion item
func (c *Completions) renderItem(item Completion, selected bool) string {
	textStyle := lipgloss.NewStyle().Width(16)
	descStyle := lipgloss.NewStyle().
		Foreground(styles.ColorMuted)

	if selected {
		textStyle = textStyle.
			Foreground(styles.ColorAccent).
			Bold(true)
		descStyle = descStyle.
			Foreground(styles.ColorSecondary)
	}

	text := textStyle.Render(item.Text)
	desc := descStyle.Render(item.Description)

	return text + " " + desc
}
