package cmd

import (
	"fmt"
	"os"

	"github.com/nchapman/llemme/internal/llama"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update llama.cpp to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking for llama.cpp updates...")
		fmt.Println()

		installed, err := llama.GetInstalledVersion()
		if err != nil {
			fmt.Printf("%s Failed to check installed version: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		release, err := llama.GetLatestVersion()
		if err != nil {
			fmt.Printf("%s Failed to get latest release: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
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
			fmt.Printf("Update to %s? [y/N] ", release.TagName)
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println(ui.Muted("Cancelled"))
				return
			}
		}

		fmt.Println()

		version, err := llama.InstallLatest()
		if err != nil {
			fmt.Printf("%s Failed to install llama.cpp: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		fmt.Printf("Updated to llama.cpp %s\n", version.TagName)
	},
}

var forceUpdate bool

func init() {
	updateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "Force update without confirmation")
	rootCmd.AddCommand(updateCmd)
}
