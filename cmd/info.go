package cmd

import (
	"fmt"
	"os"

	"github.com/nchapman/lemme/internal/config"
	"github.com/nchapman/lemme/internal/hf"
	"github.com/nchapman/lemme/internal/ui"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <user/repo>",
	Short: "Show model details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		client := hf.NewClient(cfg)
		modelRef := args[0]

		user, repo, _, err := parseModelRef(modelRef)
		if err != nil {
			fmt.Printf("%s %s\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		modelInfo, err := client.GetModel(user, repo)
		if err != nil {
			fmt.Printf("%s Failed to get model info: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		files, err := client.ListFiles(user, repo, "main")
		if err != nil {
			fmt.Printf("%s Failed to list files: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		quants := hf.ExtractQuantizations(files)

		fmt.Printf("%s\n", ui.Bold(modelInfo.ModelId))
		fmt.Printf("Author: %s\n", ui.Value(modelInfo.Author))
		fmt.Printf("License: %s\n", ui.Value(modelInfo.CardData.License))
		fmt.Printf("Created: %s\n", ui.Muted(modelInfo.CreatedAt.Format("2006-01-02")))
		fmt.Printf("Last Modified: %s\n", ui.Muted(modelInfo.LastModified.Format("2006-01-02")))

		if modelInfo.Gated {
			fmt.Printf("%s This model requires authentication\n\n", ui.Warning("⚠ "))
		}

		if len(quants) > 0 {
			fmt.Printf("\n%s\n", ui.Bold("Available Quantizations"))
			sortedQuants := hf.SortQuantizations(quants)
			for _, q := range sortedQuants {
				fmt.Printf("  • %s %s\n", ui.Bold(q.Name), ui.Muted(fmt.Sprintf("(%s)", ui.FormatBytes(q.Size))))
			}
		}

		if modelInfo.CardData.BaseModel != "" {
			fmt.Printf("\n%s\n", ui.Bold("Base Model"))
			fmt.Printf("  %s\n", ui.Value(modelInfo.CardData.BaseModel))
		}

		if len(modelInfo.Tags) > 0 {
			fmt.Printf("\n%s\n", ui.Bold("Tags"))
			for _, tag := range modelInfo.Tags {
				fmt.Printf("  • %s\n", ui.Muted(tag))
			}
		}

		fmt.Printf("\n%s\n", ui.Bold("Actions"))
		fmt.Printf("  lemme pull %s\n", ui.Value(modelRef))
		if len(quants) > 0 {
			bestQuant := hf.GetBestQuantization(quants)
			fmt.Printf("  lemme pull %s:%s\n", ui.Value(modelRef), ui.Value(bestQuant))
		}
		fmt.Printf("  lemme run %s\n", ui.Value(modelRef))
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
