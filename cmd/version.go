package cmd

import (
	"fmt"
	"runtime"

	"github.com/nchapman/lemme/internal/config"
	"github.com/nchapman/lemme/internal/llama"
	"github.com/nchapman/lemme/internal/ui"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show lemme version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(ui.Bold(fmt.Sprintf("Lemme v0.1.0 (%s/%s)", runtime.GOOS, runtime.GOARCH)))

		installed, _ := llama.GetInstalledVersion()
		if installed != nil {
			backend := "CPU"
			if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
				backend = "Metal"
			}
			fmt.Printf("llama.cpp %s (%s)\n", installed.TagName, backend)
		} else {
			fmt.Println(ui.Muted("llama.cpp not installed"))
		}

		fmt.Println()
		fmt.Println(ui.Bold("Paths:"))
		fmt.Printf("  Models:    %s\n", ui.Muted(config.ModelsPath()))
		fmt.Printf("  llama.cpp: %s\n", ui.Muted(llama.BinaryPath()))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
