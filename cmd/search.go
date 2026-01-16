package cmd

import (
	"fmt"
	"os"

	"github.com/nchapman/lemme/internal/config"
	"github.com/nchapman/lemme/internal/hf"
	"github.com/nchapman/lemme/internal/ui"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Hugging Face for GGUF models",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		client := hf.NewClient(cfg)
		query := args[0]

		results, err := client.SearchModels(query, 10)
		if err != nil {
			fmt.Printf("%s Failed to search: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Printf("No results found for '%s'\n", query)
			fmt.Println()
			fmt.Println("Tips:")
			fmt.Println("  Try a different search term")
			fmt.Println("  Check spelling")
			fmt.Println("  Browse Hugging Face: https://huggingface.co/models?library=gguf")
			os.Exit(1)
		}

		fmt.Printf("Search results for %s\n", ui.Value("\""+query+"\""))
		fmt.Println()

		table := ui.NewTable().
			AddColumn("MODEL", 40, ui.AlignLeft).
			AddColumn("DOWNLOADS", 10, ui.AlignRight).
			AddColumn("FORMAT", 12, ui.AlignLeft)

		for _, result := range results {
			modelName := fmt.Sprintf("%s/%s", result.Author, result.ModelId)
			format := result.LibraryName
			if result.Gated {
				format += " (gated)"
			}
			table.AddRow(modelName, formatNumber(result.Downloads), format)
		}

		fmt.Print(table.Render())
		fmt.Println()
		fmt.Printf("%d models found\n", len(results))
		fmt.Println()
		fmt.Println("Use 'lemme info <model>' for details")
	},
}

func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	if n < 1000000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	return fmt.Sprintf("%.1fB", float64(n)/1000000000)
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
