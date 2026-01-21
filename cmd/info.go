package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/hf"
	"github.com/nchapman/lleme/internal/ui"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:     "info <user/repo>",
	Aliases: []string{"show"},
	Short:   "Show model details",
	GroupID: "discovery",
	Args:    cobra.ExactArgs(1),
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

		fmt.Println(ui.Header(modelInfo.ModelId))
		fmt.Println()
		fmt.Printf("  %-12s %s\n", "Author", modelInfo.Author)
		if modelInfo.CardData.License != "" {
			fmt.Printf("  %-12s %s\n", "License", modelInfo.CardData.License)
		}
		fmt.Printf("  %-12s %s\n", "Updated", modelInfo.LastModified.Format("Jan 2, 2006"))

		if modelInfo.Gated {
			fmt.Println()
			fmt.Printf("  %s This model requires authentication\n", ui.Warning("!"))
		}

		if len(quants) > 0 {
			fmt.Println()
			fmt.Println(ui.Header("Quantizations"))
			fmt.Println()

			table := ui.NewTable().
				AddColumn("NAME", 12, ui.AlignLeft).
				AddColumn("SIZE", 12, ui.AlignRight)

			sortedQuants := hf.SortQuantizations(quants)
			bestQuant := hf.GetBestQuantization(quants)
			for _, q := range sortedQuants {
				size := ui.FormatBytes(q.Size)
				if q.Name == bestQuant {
					size += "  (recommended)"
				}
				table.AddRow(q.Name, size)
			}
			fmt.Print(table.Render())
		}

		if len(modelInfo.Tags) > 0 {
			fmt.Println()
			fmt.Printf("Tags: %s\n", ui.Muted(strings.Join(modelInfo.Tags, ", ")))
		}

		fmt.Println()
		fmt.Printf("  lleme pull %s\n", modelRef)
		fmt.Printf("  lleme run %s\n", modelRef)
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
