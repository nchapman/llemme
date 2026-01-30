package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/ui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:     "config",
	Short:   "Manage configuration",
	GroupID: "config",
	Long: `Manage lleme configuration.

Examples:
  lleme config edit    # Open config in $EDITOR
  lleme config show    # Print current configuration
  lleme config path    # Print config file path
  lleme config reset   # Reset config to defaults`,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open config in $EDITOR",
	Run: func(cmd *cobra.Command, args []string) {
		openConfigInEditor(config.ConfigPath())
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		printConfig()
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config file path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.ConfigPath())
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset config to defaults",
	Run: func(cmd *cobra.Command, args []string) {
		resetToDefaults(config.ConfigPath())
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "Get a config value by path",
	Long: `Get a config value using dot-separated path notation.

Examples:
  lleme config get server.port
  lleme config get huggingface.default_quant
  lleme config get llamacpp.options`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			ui.Fatal("Failed to load config: %v", err)
		}

		m, err := configToMap(cfg)
		if err != nil {
			ui.Fatal("Failed to convert config: %v", err)
		}

		val, err := getValueByPath(m, args[0])
		if err != nil {
			ui.Fatal("%v", err)
		}

		fmt.Println(formatValue(val))
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <path> <value>",
	Short: "Set a config value by path",
	Long: `Set a config value using dot-separated path notation.

Values are auto-detected as bool, int, float, or string.

Examples:
  lleme config set server.port 8080
  lleme config set llamacpp.options.ctx-size 8192
  lleme config set llamacpp.options.flash-attn true`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			ui.Fatal("Failed to load config: %v", err)
		}

		m, err := configToMap(cfg)
		if err != nil {
			ui.Fatal("Failed to convert config: %v", err)
		}

		value := parseValue(args[1])

		if err := setValueByPath(m, args[0], value); err != nil {
			ui.Fatal("%v", err)
		}

		newCfg, err := mapToConfig(m)
		if err != nil {
			ui.Fatal("Failed to convert config: %v", err)
		}

		if err := config.Save(newCfg); err != nil {
			ui.Fatal("Failed to save config: %v", err)
		}

		fmt.Printf("%s %s = %v\n", ui.Success("✓"), args[0], value)
	},
}

func resetToDefaults(path string) {
	if err := config.SaveDefault(); err != nil {
		ui.Fatal("Failed to reset config: %v", err)
	}
	fmt.Printf("%s Config reset to defaults at %s\n", ui.Success("✓"), ui.Muted(path))
}

func printConfig() {
	cfg, err := config.Load()
	if err != nil {
		ui.Fatal("Failed to load config: %v", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		ui.Fatal("Failed to format config: %v", err)
	}

	fmt.Print(string(data))
}

func openConfigInEditor(path string) {
	// Ensure config file exists with defaults if it doesn't
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		if err := config.Save(cfg); err != nil {
			ui.Fatal("Failed to create config file: %v", err)
		}
		fmt.Printf("Created default config at %s\n\n", ui.Muted(path))
	}

	if err := openInEditor(path); err != nil {
		ui.Fatal("%v", err)
	}
}

// openInEditor opens a file in the user's preferred editor.
// Returns an error if no editor is found or the editor fails.
func openInEditor(path string) error {
	editor := getEditor()
	if editor == "" {
		fmt.Printf("File location: %s\n", ui.Muted(path))
		return fmt.Errorf("no editor found - set $EDITOR or $VISUAL environment variable")
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}
	return nil
}

func getEditor() string {
	// Check VISUAL first (preferred for full-screen editors)
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	// Then EDITOR
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Fall back to common editors
	fallbacks := []string{"nano", "vim", "vi"}
	for _, editor := range fallbacks {
		if path, err := exec.LookPath(editor); err == nil {
			return path
		}
	}

	return ""
}

// parseValue auto-detects the type of a string value.
// Order: bool → int → float → string
func parseValue(s string) any {
	// Bool
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Int
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}

	// Float (only if has decimal point - ParseFloat would match ints too)
	if strings.Contains(s, ".") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}

	// String
	return s
}

// configToMap converts a Config to map[string]any via YAML round-trip.
func configToMap(cfg *config.Config) (map[string]any, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}
	return m, nil
}

// mapToConfig converts a map[string]any to Config via YAML round-trip.
func mapToConfig(m map[string]any) (*config.Config, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal map: %w", err)
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

// getValueByPath retrieves a value from a nested map using dot-separated path.
func getValueByPath(m map[string]any, path string) (any, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	parts := strings.Split(path, ".")
	var current any = m

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path not found: %s (key %q does not exist)", path, part)
			}
			current = val
		default:
			return nil, fmt.Errorf("path not found: %s (%q is not a map)", path, part)
		}
	}

	return current, nil
}

// setValueByPath sets a value in a nested map using dot-separated path.
// Creates intermediate maps as needed.
func setValueByPath(m map[string]any, path string, value any) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	parts := strings.Split(path, ".")
	current := m

	// Navigate to parent, creating intermediate maps as needed
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			// Create intermediate map
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
			continue
		}

		nextMap, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot set path: %s is not a map", strings.Join(parts[:i+1], "."))
		}
		current = nextMap
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
	return nil
}

// formatValue formats a value for display.
// Maps are formatted as YAML, other values as simple strings.
func formatValue(v any) string {
	switch val := v.(type) {
	case map[string]any:
		data, _ := yaml.Marshal(val)
		return strings.TrimSpace(string(data))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}
