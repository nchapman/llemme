package cmd

import (
	"fmt"
	"os"

	"github.com/nchapman/lemme/internal/config"
	"github.com/nchapman/lemme/internal/ui"
	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "lemme",
	Short: "Run local LLMs with Hugging Face integration",
	Long: `Lemme makes running local LLMs effortless. Point it at any GGUF 
model on Hugging Face, and it handles the rest—downloading, caching, and 
running inference through llama.cpp.`,
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
}
