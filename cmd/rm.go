package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/nchapman/lemme/internal/hf"
	"github.com/nchapman/lemme/internal/ui"
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
			if _, err := os.Stat(modelPath); os.IsNotExist(err) {
				fmt.Printf("%s Model not found: %s\n", ui.ErrorMsg("Error:"), modelRef)
				os.Exit(1)
			}

			if !force {
				confirm := false
				prompt := huh.NewConfirm().
					Title(fmt.Sprintf("Remove %s?", modelRef)).
					Description("This will delete the model file. This action cannot be undone.").
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

			fmt.Printf("%s Removed %s\n", ui.Success("✓"), modelRef)
		} else {
			if _, err := os.Stat(modelDir); os.IsNotExist(err) {
				fmt.Printf("%s Model not found: %s\n", ui.ErrorMsg("Error:"), modelRef)
				os.Exit(1)
			}

			if !force {
				confirm := false
				prompt := huh.NewConfirm().
					Title(fmt.Sprintf("Remove entire model %s?", modelRef)).
					Description("This will delete all quantizations of this model. This action cannot be undone.").
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

			fmt.Printf("%s Removed %s\n", ui.Success("✓"), modelRef)
		}
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(rmCmd)
}
