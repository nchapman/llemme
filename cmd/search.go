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
			fmt.Printf("%s No results found for '%s'\n", ui.Warning("Warning:"), query)
			fmt.Println("\nTips:")
			fmt.Println("  • Try a different search term")
			fmt.Println("  • Check spelling")
			fmt.Println("  • Browse Hugging Face: https://huggingface.co/models?library=gguf")
			os.Exit(1)
		}

		fmt.Printf("%s\n", ui.Bold("Search Results"))
		fmt.Printf("Query: %s\n", ui.Value(query))
		fmt.Printf("Found: %d models\n\n", len(results))

		for i, result := range results {
			fmt.Printf("%d. %s/%s\n", i+1, ui.Bold(result.Author), ui.Value(result.ModelId))
			fmt.Printf("   Downloads: %s\n", ui.Muted(formatNumber(result.Downloads)))
			if result.Gated {
				fmt.Printf("   %s\n", ui.Muted("(gated)"))
			}
			if result.LibraryName == "gguf" {
				fmt.Printf("   ✓ GGUF format\n")
			} else {
				fmt.Printf("   Format: %s\n", ui.Muted(result.LibraryName))
			}
			fmt.Println()
		}

		fmt.Printf("Use: lemme pull %s/<model-name>\n", ui.Bold("author"))
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
