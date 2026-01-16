package ui

import (
	"strings"
	"testing"
)

func TestStyleFunctions(t *testing.T) {
	testCases := []struct {
		name string
		fn   func(string) string
	}{
		{"Header", Header},
		{"Success", Success},
		{"ErrorMsg", ErrorMsg},
		{"Warning", Warning},
		{"Muted", Muted},
		{"Bold", Bold},
		{"Keyword", Keyword},
		{"Value", Value},
		{"Box", Box},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := "test text"
			result := tc.fn(input)

			if result == "" {
				t.Errorf("%s() returned empty string", tc.name)
			}

			if !strings.Contains(result, "test text") {
				t.Errorf("%s() result does not contain input text", tc.name)
			}
		})
	}
}

func TestStyleFunctionsEmptyInput(t *testing.T) {
	testCases := []struct {
		name string
		fn   func(string) string
	}{
		{"Header", Header},
		{"Success", Success},
		{"ErrorMsg", ErrorMsg},
		{"Warning", Warning},
		{"Muted", Muted},
		{"Bold", Bold},
		{"Keyword", Keyword},
		{"Value", Value},
		{"Box", Box},
	}

	for _, tc := range testCases {
		t.Run(tc.name+"_empty", func(t *testing.T) {
			// Should not panic on empty input
			result := tc.fn("")
			_ = result
		})
	}
}
