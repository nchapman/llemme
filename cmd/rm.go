package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/nchapman/llemme/internal/hf"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var force bool

var rmCmd = &cobra.Command{
	Use:   "rm <user/repo>[:quant]",
	Short: "Remove a downloaded model",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelRef := args[0]

		user, repo, quant, err := parseModelRef(modelRef)
		if err != nil {
			fmt.Printf("%s %s\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		modelDir := hf.GetModelPath(user, repo)

		if quant != "" {
			modelPath := hf.GetModelFilePath(user, repo, quant)
			fileInfo, err := os.Stat(modelPath)
			if os.IsNotExist(err) {
				fmt.Printf("%s Model not found: %s\n", ui.ErrorMsg("Error:"), modelRef)
				os.Exit(1)
			}

			if !force {
				confirm := false
				prompt := huh.NewConfirm().
					Title(fmt.Sprintf("Remove %s (%s)?", modelRef, ui.FormatBytes(fileInfo.Size()))).
					Value(&confirm)

				if err := prompt.Run(); err != nil {
					fmt.Printf("%s Error: %v\n", ui.ErrorMsg("Error:"), err)
					os.Exit(1)
				}

				if !confirm {
					fmt.Println(ui.Muted("Cancelled"))
					return
				}
			}

			if err := os.Remove(modelPath); err != nil {
				fmt.Printf("%s Failed to remove model: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}

			fmt.Printf("Removed %s\n", modelRef)
		} else {
			if _, err := os.Stat(modelDir); os.IsNotExist(err) {
				fmt.Printf("%s Model not found: %s\n", ui.ErrorMsg("Error:"), modelRef)
				os.Exit(1)
			}

			if !force {
				confirm := false
				prompt := huh.NewConfirm().
					Title(fmt.Sprintf("Remove all quantizations of %s?", modelRef)).
					Value(&confirm)

				if err := prompt.Run(); err != nil {
					fmt.Printf("%s Error: %v\n", ui.ErrorMsg("Error:"), err)
					os.Exit(1)
				}

				if !confirm {
					fmt.Println(ui.Muted("Cancelled"))
					return
				}
			}

			if err := os.RemoveAll(modelDir); err != nil {
				fmt.Printf("%s Failed to remove model directory: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}

			fmt.Printf("Removed %s\n", modelRef)
		}
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(rmCmd)
}
