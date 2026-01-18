package proxy

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildLlamaServerArgs(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]any
		expected map[string]string // flag -> value ("" for boolean flags)
	}{
		{
			name:     "nil config",
			config:   nil,
			expected: map[string]string{},
		},
		{
			name:     "empty config",
			config:   map[string]any{},
			expected: map[string]string{},
		},
		{
			name: "boolean true",
			config: map[string]any{
				"mlock": true,
			},
			expected: map[string]string{"mlock": ""},
		},
		{
			name: "boolean false omitted",
			config: map[string]any{
				"mlock": false,
			},
			expected: map[string]string{},
		},
		{
			name: "integer value",
			config: map[string]any{
				"parallel": 4,
			},
			expected: map[string]string{"parallel": "4"},
		},
		{
			name: "float64 whole number as int",
			config: map[string]any{
				"threads": float64(8),
			},
			expected: map[string]string{"threads": "8"},
		},
		{
			name: "float64 decimal",
			config: map[string]any{
				"temp": 0.7,
			},
			expected: map[string]string{"temp": "0.7"},
		},
		{
			name: "string value",
			config: map[string]any{
				"flash-attn": "off",
			},
			expected: map[string]string{"flash-attn": "off"},
		},
		{
			name: "empty string omitted",
			config: map[string]any{
				"api-key": "",
			},
			expected: map[string]string{},
		},
		{
			name: "multiple options",
			config: map[string]any{
				"parallel": float64(4),
				"mlock":    true,
				"threads":  float64(8),
			},
			expected: map[string]string{
				"parallel": "4",
				"mlock":    "",
				"threads":  "8",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLlamaServerArgs(tt.config)
			resultMap := parseArgsToMap(result)

			if !reflect.DeepEqual(resultMap, tt.expected) {
				t.Errorf("buildLlamaServerArgs() = %v (parsed: %v), want %v", result, resultMap, tt.expected)
			}
		})
	}
}

// parseArgsToMap converts ["--flag", "value", "--bool"] to {"flag": "value", "bool": ""}
func parseArgsToMap(args []string) map[string]string {
	m := make(map[string]string)
	for i := 0; i < len(args); i++ {
		if key, ok := strings.CutPrefix(args[i], "--"); ok {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				m[key] = args[i+1]
				i++
			} else {
				m[key] = "" // boolean flag
			}
		}
	}
	return m
}
