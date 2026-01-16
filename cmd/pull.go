package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nchapman/gollama/internal/config"
	"github.com/nchapman/gollama/internal/hf"
	"github.com/nchapman/gollama/internal/ui"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull <user/repo>[:quant]",
	Short: "Download a model from Hugging Face",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelRef := args[0]

		user, repo, quant, err := parseModelRef(modelRef)
		if err != nil {
			fmt.Printf("%s %s\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("%s Failed to load config: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		client := hf.NewClient(cfg)

		modelInfo, err := client.GetModel(user, repo)
		if err != nil {
			handleModelError(err, user, repo)
			os.Exit(1)
		}

		if modelInfo.Gated && cfg.HFToken == "" && os.Getenv("HF_TOKEN") == "" {
			fmt.Printf("%s\n", ui.ErrorMsg("Error: Authentication required"))
			fmt.Printf("\nThe repository '%s/%s' requires authentication.\n\n", user, repo)
			fmt.Println("To access gated models, provide a Hugging Face token:")
			fmt.Println("  1. Get a token at https://huggingface.co/settings/tokens")
			fmt.Println("  2. Run: huggingface-cli login")
			fmt.Println("     Or set: export HF_TOKEN=hf_xxxxx")
			os.Exit(1)
		}

		files, err := client.ListFiles(user, repo, "main")
		if err != nil {
			fmt.Printf("%s Failed to list files: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		quants := hf.ExtractQuantizations(files)
		if len(quants) == 0 {
			fmt.Printf("%s No GGUF files found\n", ui.ErrorMsg("Error:"))
			fmt.Printf("\nThe repository '%s/%s' exists but contains no GGUF files.\n\n", user, repo)
			fmt.Println("Try one of these GGUF versions:")
			fmt.Printf("  • %s/%s\n", user+"GGUF", repo)
			os.Exit(1)
		}

		if quant == "" {
			quant = hf.GetBestQuantization(quants)
		} else {
			_, found := hf.FindQuantization(quants, quant)
			if !found {
				fmt.Printf("%s Quantization '%s' not found\n", ui.ErrorMsg("Error:"), quant)
				fmt.Println("\nAvailable quantizations:")
				for _, q := range hf.SortQuantizations(quants) {
					fmt.Printf("  • %s (%s)\n", q.Name, ui.FormatBytes(q.Size))
				}
				os.Exit(1)
			}
		}

		modelDir := hf.GetModelPath(user, repo)
		modelPath := hf.GetModelFilePath(user, repo, quant)

		if _, err := os.Stat(modelPath); err == nil {
			fmt.Printf("Model already downloaded: %s\n", ui.Bold(modelPath))
			return
		}

		if err := os.MkdirAll(modelDir, 0755); err != nil {
			fmt.Printf("%s Failed to create model directory: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		selectedQuant, _ := hf.FindQuantization(quants, quant)
		message := fmt.Sprintf("Pulling %s/%s:%s", user, repo, quant)
		fmt.Printf("%s\n", ui.Bold(message))
		fmt.Printf("  Model info:\n")
		fmt.Printf("    • Size: %s\n", ui.FormatBytes(selectedQuant.Size))
		fmt.Println()

		progressBar := ui.NewProgressBar("", selectedQuant.Size)
		progressBar.Start(message, selectedQuant.Size)

		downloaderWithProgress := hf.NewDownloaderWithProgress(client, func(downloaded, total int64, speed float64, eta time.Duration) {
			progressBar.Update(downloaded)
		})

		_, err = downloaderWithProgress.DownloadModel(user, repo, "main", selectedQuant.File, modelPath)
		if err != nil {
			progressBar.Stop()
			fmt.Printf("%s Failed to download: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		progressBar.Finish("Pulled " + user + "/" + repo + ":" + quant + " successfully!")
		fmt.Println()
	},
}

func parseModelRef(ref string) (user, repo, quant string, err error) {
	parts := strings.Split(ref, ":")
	if len(parts) > 2 {
		return "", "", "", fmt.Errorf("invalid model reference: %s", ref)
	}

	mainRef := parts[0]
	quantPart := ""
	if len(parts) == 2 {
		quantPart = parts[1]
	}

	repoParts := strings.Split(mainRef, "/")
	if len(repoParts) != 2 {
		return "", "", "", fmt.Errorf("model reference must be in format user/repo: %s", ref)
	}

	return repoParts[0], repoParts[1], quantPart, nil
}

func handleModelError(err error, user, repo string) {
	errStr := err.Error()

	if strings.Contains(errStr, "404") {
		fmt.Printf("%s Model not found\n", ui.ErrorMsg("Error:"))
		fmt.Printf("\nCould not find '%s/%s' on Hugging Face.\n\n", user, repo)
		fmt.Println("Tips:")
		fmt.Println("  • Check the spelling of the repository name")
		fmt.Println("  • Use 'gollama search <query>' to find models")
	} else {
		fmt.Printf("%s %v\n", ui.ErrorMsg("Error:"), err)
	}
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
