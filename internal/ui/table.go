package ui

import (
	"fmt"
	"strings"
)

// Alignment specifies how text should be aligned within a column.
type Alignment int

const (
	AlignLeft Alignment = iota
	AlignRight
)

// Column defines a table column with a header, width, and alignment.
type Column struct {
	Header string
	Width  int
	Align  Alignment
}

// Table renders tabular data with consistent formatting.
type Table struct {
	columns []Column
	rows    [][]string
	indent  int
}

// NewTable creates a new table with default settings.
func NewTable() *Table {
	return &Table{
		indent: 2,
	}
}

// Indent sets the left indentation for the table.
func (t *Table) Indent(spaces int) *Table {
	t.indent = spaces
	return t
}

// AddColumn adds a column to the table.
func (t *Table) AddColumn(header string, width int, align Alignment) *Table {
	t.columns = append(t.columns, Column{
		Header: header,
		Width:  width,
		Align:  align,
	})
	return t
}

// AddRow adds a row of values to the table.
func (t *Table) AddRow(values ...string) *Table {
	t.rows = append(t.rows, values)
	return t
}

// formatCell formats a single cell value according to width and alignment.
func (t *Table) formatCell(value string, col Column) string {
	// Truncate if too long
	if len(value) > col.Width {
		if col.Width <= 3 {
			value = value[:col.Width]
		} else {
			value = value[:col.Width-3] + "..."
		}
	}

	// Pad according to alignment
	if col.Align == AlignRight {
		return fmt.Sprintf("%*s", col.Width, value)
	}
	return fmt.Sprintf("%-*s", col.Width, value)
}

// Render returns the formatted table as a string.
func (t *Table) Render() string {
	if len(t.columns) == 0 {
		return ""
	}

	var b strings.Builder
	indent := strings.Repeat(" ", t.indent)
	gap := "  " // 2-space column gaps

	// Render header
	b.WriteString(indent)
	for i, col := range t.columns {
		if i > 0 {
			b.WriteString(gap)
		}
		cell := t.formatCell(col.Header, col)
		b.WriteString(Header(cell))
	}
	b.WriteString("\n")

	// Render rows
	for _, row := range t.rows {
		b.WriteString(indent)
		for i, col := range t.columns {
			if i > 0 {
				b.WriteString(gap)
			}
			value := ""
			if i < len(row) {
				value = row[i]
			}
			b.WriteString(t.formatCell(value, col))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// String implements the Stringer interface.
func (t *Table) String() string {
	return t.Render()
}
