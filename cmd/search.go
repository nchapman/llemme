package cmd

import (
	"fmt"
	"os"

	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/hf"
	"github.com/nchapman/llemme/internal/proxy"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:     "search [query]",
	Short:   "Search Hugging Face for llama.cpp compatible models",
	GroupID: "discovery",
	Long:    "Search Hugging Face for llama.cpp compatible models. If no query is provided, shows trending models.",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		client := hf.NewClient(cfg)
		query := ""
		if len(args) > 0 {
			query = args[0]
		}

		results, err := client.SearchModels(query, 15)
		if err != nil {
			fmt.Printf("%s Failed to search: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if len(results) == 0 {
			if query != "" {
				fmt.Printf("No results found for '%s'\n", query)
			} else {
				fmt.Println("No models found")
			}
			fmt.Println()
			fmt.Println("Tips:")
			fmt.Println("  Try a different search term")
			fmt.Println("  Check spelling")
			fmt.Println("  Browse Hugging Face: https://huggingface.co/models?apps=llama.cpp")
			os.Exit(1)
		}

		// Build set of installed models (by user/repo).
		// Errors are ignored since install indicators are non-critical UI hints.
		installed := make(map[string]bool)
		resolver := proxy.NewModelResolver()
		if downloaded, err := resolver.ListDownloadedModels(); err == nil {
			for _, m := range downloaded {
				installed[m.User+"/"+m.Repo] = true
			}
		}

		table := ui.NewTable().
			Indent(0).
			AddColumn("MODEL", 54, ui.AlignLeft).
			AddColumn("DOWNLOADS", 10, ui.AlignRight).
			AddColumn("LIKES", 8, ui.AlignRight)

		for _, result := range results {
			indicator := "○"
			if installed[result.ID] {
				indicator = "✓"
			}
			modelName := indicator + " " + result.ID
			if result.Gated {
				modelName += " (gated)"
			}
			table.AddRow(modelName, formatNumber(result.Downloads), formatNumber(result.Likes))
		}

		fmt.Print(table.Render())
		fmt.Println("\n✓ = installed")
		fmt.Printf("%d results from Hugging Face\n", len(results))
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
