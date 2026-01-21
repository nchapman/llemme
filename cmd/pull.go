package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/hf"
	"github.com/nchapman/lleme/internal/ui"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:     "pull <user/repo>[:quant]",
	Short:   "Download a model from Hugging Face",
	GroupID: "model",
	Args:    cobra.ExactArgs(1),
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

		if modelInfo.Gated && cfg.HuggingFace.Token == "" && os.Getenv("HF_TOKEN") == "" {
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

		if err := os.MkdirAll(modelDir, 0755); err != nil {
			fmt.Printf("%s Failed to create model directory: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		// Fetch remote manifest to check for updates and get file info
		manifest, manifestJSON, err := client.GetManifest(user, repo, quant)
		if err != nil {
			fmt.Printf("%s Failed to get manifest: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if manifest.GGUFFile == nil {
			fmt.Printf("%s Manifest does not contain a GGUF file\n", ui.ErrorMsg("Error:"))
			os.Exit(1)
		}

		// Check if local files are up to date with remote manifest
		upToDate, saveManifest := isUpToDate(user, repo, quant, manifest)
		if upToDate {
			if saveManifest {
				// Legacy model without manifest - save it now
				manifestPath := hf.GetManifestFilePath(user, repo, quant)
				os.WriteFile(manifestPath, manifestJSON, 0644)
			}
			fmt.Printf("Model is up to date: %s\n", ui.Bold(modelPath))
			return
		}

		// Calculate total download size
		totalSize := manifest.GGUFFile.Size
		hasMMProj := manifest.MMProjFile != nil
		if hasMMProj {
			totalSize += manifest.MMProjFile.Size
		}

		modelName := ui.Keyword(fmt.Sprintf("%s/%s:%s", user, repo, quant))
		if hasMMProj {
			fmt.Printf("Pulling %s (%s + %s mmproj)\n",
				modelName,
				ui.FormatBytes(manifest.GGUFFile.Size),
				ui.FormatBytes(manifest.MMProjFile.Size))
		} else {
			fmt.Printf("Pulling %s (%s)\n", modelName, ui.FormatBytes(manifest.GGUFFile.Size))
		}

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
		var mmprojPath string
		if hasMMProj {
			mmprojPath = hf.GetMMProjFilePath(user, repo, quant)
			_, err = downloaderWithProgress.DownloadModel(user, repo, "main", manifest.MMProjFile.RFilename, mmprojPath)
			if err != nil {
				progressBar.Stop()
				fmt.Printf("%s Failed to download mmproj: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
		}

		progressBar.Finish("Downloaded")

		// Verify downloaded files against manifest hashes
		if manifest.GGUFFile.LFS != nil {
			verifyBar := ui.NewProgressBar()
			verifyBar.Start("Verifying", totalSize)
			verified := int64(0)

			hash, err := hf.CalculateSHA256WithProgress(modelPath, func(processed, total int64) {
				verifyBar.Update(verified + processed)
			})
			if err != nil {
				verifyBar.Stop()
				fmt.Printf("%s Failed to verify model: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
			if hash != manifest.GGUFFile.LFS.SHA256 {
				verifyBar.Stop()
				fmt.Printf("%s Model verification failed: hash mismatch\n", ui.ErrorMsg("Error:"))
				os.Remove(modelPath)
				os.Exit(1)
			}
			verified += manifest.GGUFFile.Size

			if hasMMProj && manifest.MMProjFile.LFS != nil {
				hash, err := hf.CalculateSHA256WithProgress(mmprojPath, func(processed, total int64) {
					verifyBar.Update(verified + processed)
				})
				if err != nil {
					verifyBar.Stop()
					fmt.Printf("%s Failed to verify mmproj: %v\n", ui.ErrorMsg("Error:"), err)
					os.Exit(1)
				}
				if hash != manifest.MMProjFile.LFS.SHA256 {
					verifyBar.Stop()
					fmt.Printf("%s mmproj verification failed: hash mismatch\n", ui.ErrorMsg("Error:"))
					os.Remove(mmprojPath)
					os.Exit(1)
				}
			}

			verifyBar.Finish("Verified")
		}

		// Save manifest for offline reference and verification
		manifestPath := hf.GetManifestFilePath(user, repo, quant)
		if err := os.WriteFile(manifestPath, manifestJSON, 0644); err != nil {
			fmt.Printf("%s Failed to save manifest: %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if hasMMProj {
			fmt.Printf("Pulled %s (vision model)\n", modelName)
		} else {
			fmt.Printf("Pulled %s\n", modelName)
		}
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

// isUpToDate checks if local files match the remote manifest by comparing sha256 hashes.
// Returns (up-to-date, should-save-manifest).
func isUpToDate(user, repo, quant string, remote *hf.Manifest) (bool, bool) {
	manifestPath := hf.GetManifestFilePath(user, repo, quant)
	modelPath := hf.GetModelFilePath(user, repo, quant)

	// Load saved manifest
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		// No local manifest - check if this is a legacy download we can upgrade
		modelInfo, statErr := os.Stat(modelPath)
		if statErr == nil && modelInfo.Size() == remote.GGUFFile.Size {
			// Model exists with matching size - likely a legacy download
			// Check if vision model needs mmproj
			if remote.MMProjFile != nil {
				mmprojPath := hf.GetMMProjFilePath(user, repo, quant)
				if _, err := os.Stat(mmprojPath); err != nil {
					return false, false // Need to download mmproj
				}
			}
			// Save manifest for this legacy model and consider it up to date
			return true, true
		}
		return false, false
	}

	var local hf.Manifest
	if err := json.Unmarshal(manifestData, &local); err != nil {
		return false, false // Can't parse, need to download
	}

	// Compare GGUF file hash
	if !hashesMatch(local.GGUFFile, remote.GGUFFile) {
		return false, false
	}

	// Compare mmproj hash if remote has one
	if remote.MMProjFile != nil {
		if !hashesMatch(local.MMProjFile, remote.MMProjFile) {
			return false, false
		}
		// Also verify the mmproj file actually exists
		mmprojPath := hf.GetMMProjFilePath(user, repo, quant)
		if _, err := os.Stat(mmprojPath); err != nil {
			return false, false
		}
	}

	// Verify the model file actually exists
	if _, err := os.Stat(modelPath); err != nil {
		return false, false
	}

	return true, false
}

// hashesMatch compares the sha256 hashes of two manifest files.
func hashesMatch(local, remote *hf.ManifestFile) bool {
	if local == nil || remote == nil {
		return local == nil && remote == nil
	}
	if local.LFS == nil || remote.LFS == nil {
		// No hash info, fall back to size comparison
		return local.Size == remote.Size
	}
	return local.LFS.SHA256 == remote.LFS.SHA256
}

func handleModelError(err error, user, repo string) {
	errStr := err.Error()

	if strings.Contains(errStr, "404") {
		fmt.Printf("%s Model not found\n", ui.ErrorMsg("Error:"))
		fmt.Printf("\nCould not find '%s/%s' on Hugging Face.\n\n", user, repo)
		fmt.Println("Tips:")
		fmt.Println("  • Check the spelling of the repository name")
		fmt.Println("  • Use 'lleme search <query>' to find models")
	} else {
		fmt.Printf("%s %v\n", ui.ErrorMsg("Error:"), err)
	}
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
