package cmd

import (
	"fmt"
	"os"

	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	// Version is set via ldflags at build time
	Version = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "llemme",
	Short:   "Run local LLMs with llama.cpp and Hugging Face",
	Version: Version,
	Long: `Run local LLMs with llama.cpp and Hugging Face.

Point it at any GGUF model on Hugging Face, and it handles the rest—downloading,
caching, and running inference.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		ui.InitLogger(verbose)
		if err := config.EnsureDirectories(); err != nil {
			fmt.Printf("Error: Failed to create directories: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "model", Title: "Model Commands:"},
		&cobra.Group{ID: "discovery", Title: "Discovery:"},
		&cobra.Group{ID: "server", Title: "Server:"},
		&cobra.Group{ID: "config", Title: "Configuration:"},
	)
}
