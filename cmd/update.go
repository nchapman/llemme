package cmd

import (
	"fmt"
	"os"

	"github.com/nchapman/lemme/internal/llama"
	"github.com/nchapman/lemme/internal/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update llama.cpp to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking for updates...")
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

		fmt.Printf("  Current: %s\n", ui.Muted(currentVersion))
		fmt.Printf("  Latest:  %s\n", ui.Muted(release.TagName))
		fmt.Println()

		if installed != nil && installed.TagName == release.TagName {
			fmt.Println(ui.Success("llama.cpp is already up to date (" + release.TagName + ")"))
			return
		}

		if !forceUpdate {
			fmt.Printf("Update llama.cpp from %s to %s? [y/N] ", currentVersion, release.TagName)
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println(ui.Muted("Cancelled"))
				return
			}
		}

		version, err := llama.InstallLatest()
		if err != nil {
			fmt.Printf("%s Failed to install llama.cpp: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Printf("%s Updated successfully to %s!\n", ui.Success("✓"), version.TagName)
	},
}

var forceUpdate bool

func init() {
	updateCmd.Flags().BoolVarP(&forceUpdate, "force", "f", false, "Force update without confirmation")
	rootCmd.AddCommand(updateCmd)
}
