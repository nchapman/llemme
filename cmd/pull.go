package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/hf"
	"github.com/nchapman/llemme/internal/ui"
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

		// Check if model is already complete by verifying against saved manifest
		if isModelComplete(user, repo, quant) {
			fmt.Printf("Model already downloaded: %s\n", ui.Bold(modelPath))
			return
		}

		if err := os.MkdirAll(modelDir, 0755); err != nil {
			fmt.Printf("%s Failed to create model directory: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		// Get manifest to find exact filenames and check for mmproj (vision model support)
		manifest, manifestJSON, err := client.GetManifest(user, repo, quant)
		if err != nil {
			fmt.Printf("%s Failed to get manifest: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if manifest.GGUFFile == nil {
			fmt.Printf("%s Manifest does not contain a GGUF file\n", ui.ErrorMsg("Error:"))
			os.Exit(1)
		}

		// Calculate total download size
		totalSize := manifest.GGUFFile.Size
		hasMMProj := manifest.MMProjFile != nil
		if hasMMProj {
			totalSize += manifest.MMProjFile.Size
		}

		if hasMMProj {
			fmt.Printf("Pulling %s/%s:%s (%s + %s mmproj)\n",
				user, repo, quant,
				ui.FormatBytes(manifest.GGUFFile.Size),
				ui.FormatBytes(manifest.MMProjFile.Size))
		} else {
			fmt.Printf("Pulling %s/%s:%s (%s)\n", user, repo, quant, ui.FormatBytes(manifest.GGUFFile.Size))
		}
		fmt.Println()

		progressBar := ui.NewProgressBar()
		progressBar.Start("", totalSize)

		downloaded := int64(0)
		downloaderWithProgress := hf.NewDownloaderWithProgress(client, func(current, total int64, speed float64, eta time.Duration) {
			progressBar.Update(downloaded + current)
		})

		// Download main model
		_, err = downloaderWithProgress.DownloadModel(user, repo, "main", manifest.GGUFFile.RFilename, modelPath)
		if err != nil {
			progressBar.Stop()
			fmt.Printf("%s Failed to download model: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}
		downloaded += manifest.GGUFFile.Size

		// Download mmproj if present (vision model)
		if hasMMProj {
			mmprojPath := hf.GetMMProjFilePath(user, repo, quant)
			_, err = downloaderWithProgress.DownloadModel(user, repo, "main", manifest.MMProjFile.RFilename, mmprojPath)
			if err != nil {
				progressBar.Stop()
				fmt.Printf("%s Failed to download mmproj: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
		}

		// Save manifest for offline reference and verification
		manifestPath := hf.GetManifestFilePath(user, repo, quant)
		if err := os.WriteFile(manifestPath, manifestJSON, 0644); err != nil {
			progressBar.Stop()
			fmt.Printf("%s Failed to save manifest: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if hasMMProj {
			progressBar.Finish(fmt.Sprintf("Downloaded %s/%s:%s (vision model)", user, repo, quant))
		} else {
			progressBar.Finish(fmt.Sprintf("Downloaded %s/%s:%s", user, repo, quant))
		}
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

// isModelComplete checks if all expected files exist based on the saved manifest.
// If no manifest exists, falls back to just checking if the model file exists.
func isModelComplete(user, repo, quant string) bool {
	modelPath := hf.GetModelFilePath(user, repo, quant)
	manifestPath := hf.GetManifestFilePath(user, repo, quant)

	// Check if model file exists
	if _, err := os.Stat(modelPath); err != nil {
		return false
	}

	// Try to load the saved manifest
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		// No manifest - legacy download, trust that model exists
		return true
	}

	// Parse manifest to check if mmproj is expected
	var manifest hf.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		// Can't parse manifest - trust that model exists
		return true
	}

	// If manifest specifies an mmproj file, verify it exists
	if manifest.MMProjFile != nil {
		mmprojPath := hf.GetMMProjFilePath(user, repo, quant)
		if _, err := os.Stat(mmprojPath); err != nil {
			return false
		}
	}

	return true
}

func handleModelError(err error, user, repo string) {
	errStr := err.Error()

	if strings.Contains(errStr, "404") {
		fmt.Printf("%s Model not found\n", ui.ErrorMsg("Error:"))
		fmt.Printf("\nCould not find '%s/%s' on Hugging Face.\n\n", user, repo)
		fmt.Println("Tips:")
		fmt.Println("  • Check the spelling of the repository name")
		fmt.Println("  • Use 'llemme search <query>' to find models")
	} else {
		fmt.Printf("%s %v\n", ui.ErrorMsg("Error:"), err)
	}
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
