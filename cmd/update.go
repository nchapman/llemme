package cmd

import (
	"fmt"

	"github.com/nchapman/lleme/internal/llama"
	"github.com/nchapman/lleme/internal/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "update",
	Short:   "Update llama.cpp to the latest version",
	GroupID: "config",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking for llama.cpp updates...")
		fmt.Println()

		installed, err := llama.GetInstalledVersion()
		if err != nil {
			ui.Fatal("Failed to check installed version: %v", err)
		}

		release, err := llama.GetLatestVersion()
		if err != nil {
			ui.Fatal("Failed to get latest release: %v", err)
		}

		currentVersion := "Not installed"
		if installed != nil {
			currentVersion = installed.TagName
		}

		fmt.Printf("  %-12s %s\n", "Installed", currentVersion)
		fmt.Printf("  %-12s %s\n", "Available", release.TagName)
		fmt.Println()

		if installed != nil && installed.TagName == release.TagName {
			fmt.Println("llama.cpp is already up to date")
			return
		}

		if !forceUpdate {
			if !ui.PromptYesNo(fmt.Sprintf("Update to %s?", release.TagName), false) {
				fmt.Println(ui.Muted("Cancelled"))
				return
			}
		}

		fmt.Println()

		version, err := llama.InstallLatest(func(msg string) { fmt.Println(msg) })
		if err != nil {
			ui.Fatal("Failed to install llama.cpp: %v", err)
		}

		fmt.Printf("Updated to llama.cpp %s\n", version.TagName)
	},
}

var forceUpdate bool

func init() {
	updateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "Skip confirmation")
	rootCmd.AddCommand(updateCmd)
}
