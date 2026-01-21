package cmd

import (
	"fmt"
	"os"
	"os/exec"

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
		openInEditor(config.ConfigPath())
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

func openInEditor(path string) {
	// Ensure config file exists with defaults if it doesn't
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		if err := config.Save(cfg); err != nil {
			ui.Fatal("Failed to create config file: %v", err)
		}
		fmt.Printf("Created default config at %s\n\n", ui.Muted(path))
	}

	editor := getEditor()
	if editor == "" {
		ui.PrintError("No editor found. Set $EDITOR or $VISUAL environment variable.")
		fmt.Printf("\nConfig file location: %s\n", ui.Muted(path))
		os.Exit(1)
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		ui.Fatal("Failed to open editor: %v", err)
	}
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

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configResetCmd)
}
