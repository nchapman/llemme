package components

import "testing"

func TestCompletions_OpenClose(t *testing.T) {
	c := NewCompletions()

	if c.IsOpen() {
		t.Error("expected completions to be closed initially")
	}

	items := []Completion{
		{Text: "/help", Description: "Show help", Value: "/help"},
		{Text: "/clear", Description: "Clear chat", Value: "/clear"},
	}
	c.Open(items)

	if !c.IsOpen() {
		t.Error("expected completions to be open after Open()")
	}
	if c.Count() != 2 {
		t.Errorf("expected 2 items, got %d", c.Count())
	}

	c.Close()

	if c.IsOpen() {
		t.Error("expected completions to be closed after Close()")
	}
	if c.Count() != 0 {
		t.Errorf("expected 0 items after close, got %d", c.Count())
	}
}

func TestCompletions_Filter(t *testing.T) {
	items := []Completion{
		{Text: "/help", Description: "Show help", Value: "/help"},
		{Text: "/clear", Description: "Clear chat", Value: "/clear"},
		{Text: "/bye", Description: "Exit", Value: "/bye"},
		{Text: "/set", Description: "Set option", Value: "/set"},
	}

	tests := []struct {
		name          string
		query         string
		expectedCount int
		shouldClose   bool
	}{
		{"empty query shows all", "", 4, false},
		{"filter by prefix /h", "/h", 1, false},
		{"filter by prefix /c", "/c", 1, false},
		{"filter by prefix /", "/", 4, false},
		{"filter by prefix /s", "/s", 1, false},
		{"no matches closes", "/xyz", 0, true},
		{"case insensitive", "/HELP", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCompletions()
			c.Open(items)
			c.Filter(tt.query)

			if tt.shouldClose {
				if c.IsOpen() {
					t.Error("expected completions to close with no matches")
				}
			} else {
				if c.Count() != tt.expectedCount {
					t.Errorf("expected %d items, got %d", tt.expectedCount, c.Count())
				}
			}
		})
	}
}

func TestCompletions_Navigation(t *testing.T) {
	items := []Completion{
		{Text: "/help", Value: "/help"},
		{Text: "/clear", Value: "/clear"},
		{Text: "/bye", Value: "/bye"},
	}

	c := NewCompletions()
	c.Open(items)

	// Initial selection is first item
	sel := c.Selected()
	if sel == nil || sel.Value != "/help" {
		t.Errorf("expected initial selection to be /help, got %v", sel)
	}

	// Move down
	c.MoveDown()
	sel = c.Selected()
	if sel == nil || sel.Value != "/clear" {
		t.Errorf("expected /clear after MoveDown, got %v", sel)
	}

	// Move down again
	c.MoveDown()
	sel = c.Selected()
	if sel == nil || sel.Value != "/bye" {
		t.Errorf("expected /bye after second MoveDown, got %v", sel)
	}

	// Move down wraps to first
	c.MoveDown()
	sel = c.Selected()
	if sel == nil || sel.Value != "/help" {
		t.Errorf("expected /help after wrap, got %v", sel)
	}

	// Move up wraps to last
	c.MoveUp()
	sel = c.Selected()
	if sel == nil || sel.Value != "/bye" {
		t.Errorf("expected /bye after MoveUp wrap, got %v", sel)
	}
}

func TestCompletions_FilterResetsSelection(t *testing.T) {
	items := []Completion{
		{Text: "/help", Value: "/help"},
		{Text: "/hello", Value: "/hello"},
		{Text: "/clear", Value: "/clear"},
	}

	c := NewCompletions()
	c.Open(items)

	// Move selection to second item
	c.MoveDown()
	sel := c.Selected()
	if sel == nil || sel.Value != "/hello" {
		t.Fatalf("expected /hello, got %v", sel)
	}

	// Filter to only /clear - selection should reset
	c.Filter("/c")
	if c.Count() != 1 {
		t.Errorf("expected 1 item, got %d", c.Count())
	}

	sel = c.Selected()
	if sel == nil || sel.Value != "/clear" {
		t.Errorf("expected /clear selected after filter, got %v", sel)
	}
}

func TestCompletions_SelectedEmpty(t *testing.T) {
	c := NewCompletions()

	// No items opened
	if sel := c.Selected(); sel != nil {
		t.Errorf("expected nil selected when closed, got %v", sel)
	}

	// Open empty items
	c.Open([]Completion{})
	if sel := c.Selected(); sel != nil {
		t.Errorf("expected nil selected with empty items, got %v", sel)
	}
}

func TestCompletions_MoveOnEmpty(t *testing.T) {
	c := NewCompletions()
	c.Open([]Completion{})

	// Should not panic
	c.MoveUp()
	c.MoveDown()

	if sel := c.Selected(); sel != nil {
		t.Errorf("expected nil after move on empty, got %v", sel)
	}
}

func TestCompletions_FilterMatchesByTextAndValue(t *testing.T) {
	items := []Completion{
		{Text: "Display Text", Description: "desc", Value: "/actual-value"},
		{Text: "/visible", Description: "desc", Value: "hidden-value"},
	}

	c := NewCompletions()
	c.Open(items)

	// Should match by Value prefix
	c.Filter("/actual")
	if c.Count() != 1 {
		t.Errorf("expected filter to match by value, got count %d", c.Count())
	}

	// Should also match by Text prefix
	c.Close()
	c.Open(items)
	c.Filter("/visible")
	if c.Count() != 1 {
		t.Errorf("expected filter to match by text, got count %d", c.Count())
	}
}

func TestCompletions_FilterCaching(t *testing.T) {
	items := []Completion{
		{Text: "/help", Value: "/help"},
		{Text: "/clear", Value: "/clear"},
	}

	c := NewCompletions()
	c.Open(items)

	// Filter with same query multiple times - should be idempotent
	c.Filter("/h")
	count1 := c.Count()

	c.Filter("/h")
	count2 := c.Count()

	if count1 != count2 {
		t.Errorf("expected same count on repeated filter, got %d then %d", count1, count2)
	}
}

func TestCompletions_FilterAdjustsSelectionWhenOutOfBounds(t *testing.T) {
	items := []Completion{
		{Text: "/aaa", Value: "/aaa"},
		{Text: "/bbb", Value: "/bbb"},
		{Text: "/ccc", Value: "/ccc"},
	}

	c := NewCompletions()
	c.Open(items)

	// Move to last item (index 2)
	c.MoveDown()
	c.MoveDown()
	sel := c.Selected()
	if sel == nil || sel.Value != "/ccc" {
		t.Fatalf("expected selection at /ccc, got %v", sel)
	}

	// Filter to only one item - selection should adjust to valid index
	c.Filter("/aaa")

	if c.Count() != 1 {
		t.Fatalf("expected 1 item, got %d", c.Count())
	}

	sel = c.Selected()
	if sel == nil || sel.Value != "/aaa" {
		t.Errorf("expected selection adjusted to /aaa, got %v", sel)
	}
}

func TestCompletions_ViewNotEmpty(t *testing.T) {
	items := []Completion{
		{Text: "/help", Value: "/help", Description: "Show help"},
		{Text: "/clear", Value: "/clear", Description: "Clear chat"},
	}

	c := NewCompletions()
	c.Open(items)

	view := c.View()
	if view == "" {
		t.Error("expected non-empty view when completions are open")
	}

	// Closed completions should return empty
	c.Close()
	view = c.View()
	if view != "" {
		t.Errorf("expected empty view when closed, got '%s'", view)
	}
}
