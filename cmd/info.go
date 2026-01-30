package cmd

import (
	"fmt"
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
			ui.Fatal("Failed to load config: %v", err)
		}

		client := hf.NewClient(cfg)
		modelRef := args[0]

		user, repo, _, err := parseModelRef(modelRef)
		if err != nil {
			ui.Fatal("%s", err)
		}

		modelInfo, err := client.GetModel(user, repo)
		if err != nil {
			ui.Fatal("Failed to get model info: %v", err)
		}

		files, err := client.ListFiles(user, repo, "main")
		if err != nil {
			ui.Fatal("Failed to list files: %v", err)
		}

		quants := hf.ExtractQuantizations(files)
		client.FetchFolderQuantSizes(user, repo, "main", quants)

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
			for _, q := range sortedQuants {
				table.AddRow(q.Name, ui.FormatBytes(q.Size))
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
