package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/hf"
	"github.com/nchapman/lleme/internal/peer"
	"github.com/nchapman/lleme/internal/ui"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:     "pull <user/repo>[:quant]",
	Short:   "Download a model from Hugging Face",
	GroupID: "model",
	Long: `Download a model from Hugging Face.

Examples:
  lleme pull unsloth/Llama-3.2-1B-Instruct-GGUF           # Download default quant
  lleme pull unsloth/Llama-3.2-1B-Instruct-GGUF:Q8_0      # Download specific quant`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelRef := args[0]

		user, repo, quant, err := parseModelRef(modelRef)
		if err != nil {
			ui.Fatal("%s", err)
		}

		cfg, err := config.Load()
		if err != nil {
			ui.Fatal("Failed to load config: %v", err)
		}

		client := hf.NewClient(cfg)

		modelInfo, err := client.GetModel(user, repo)
		if err != nil {
			handleModelError(err, user, repo)
			os.Exit(1)
		}

		if bool(modelInfo.Gated) && !hf.HasToken(cfg) {
			ui.PrintError("Authentication required")
			fmt.Printf("\nThe repository '%s/%s' requires authentication.\n\n", user, repo)
			fmt.Println("To access gated models, provide a Hugging Face token:")
			fmt.Println("  1. Get a token at https://huggingface.co/settings/tokens")
			fmt.Println("  2. Run: hf auth login")
			fmt.Println("     Or set: export HF_TOKEN=hf_xxxxx")
			os.Exit(1)
		}

		files, err := client.ListFiles(user, repo, "main")
		if err != nil {
			ui.Fatal("Failed to list files: %v", err)
		}

		quants := hf.ExtractQuantizations(files)
		if len(quants) == 0 {
			ui.PrintError("No GGUF files found")
			fmt.Printf("\nThe repository '%s/%s' exists but contains no GGUF files.\n", user, repo)
			os.Exit(1)
		}

		// Find the quantization to use
		var selectedQuant hf.Quantization
		if quant == "" {
			quant = hf.GetBestQuantization(quants)
			selectedQuant, _ = hf.FindQuantization(quants, quant)
		} else {
			var found bool
			selectedQuant, found = hf.FindQuantization(quants, quant)
			if !found {
				ui.PrintError("Quantization '%s' not found", quant)
				fmt.Println("\nAvailable quantizations:")
				client.FetchFolderQuantSizes(user, repo, "main", quants)
				for _, q := range hf.SortQuantizations(quants) {
					fmt.Printf("  • %s (%s)\n", q.Name, ui.FormatBytes(q.Size))
				}
				os.Exit(1)
			}
		}

		// Check if local files are up to date with remote manifest
		upToDate, saveManifest, manifest, manifestJSON, err := hf.CheckForUpdates(client, user, repo, selectedQuant)
		if err != nil {
			ui.Fatal("%v", err)
		}
		if upToDate {
			if saveManifest {
				// Legacy model without manifest - save it now
				manifestPath := hf.GetManifestFilePath(user, repo, quant)
				if err := os.WriteFile(manifestPath, manifestJSON, 0644); err != nil {
					ui.Fatal("Failed to save manifest: %v", err)
				}
			}
			// Find the actual model path (handles both single and split files)
			modelPath := hf.FindModelFile(user, repo, quant)
			if modelPath == "" {
				modelPath = hf.GetModelFilePath(user, repo, quant) // Fallback for display
			}
			fmt.Printf("Model is up to date: %s\n", ui.Bold(modelPath))
			return
		}

		// Try to pull from a peer first if peer discovery is enabled
		// Only works for single files with a hash (not split files)
		if cfg.Peer.Enabled && manifest != nil && manifest.GGUFFile != nil &&
			manifest.GGUFFile.LFS != nil && manifest.GGUFFile.LFS.SHA256 != "" {
			hash := manifest.GGUFFile.LFS.SHA256
			if pulledFromPeer := tryPullFromPeer(user, repo, selectedQuant.Name, hash); pulledFromPeer {
				// Save manifest for peer-downloaded model
				manifestPath := hf.GetManifestFilePath(user, repo, selectedQuant.Name)
				if err := os.WriteFile(manifestPath, manifestJSON, 0644); err != nil {
					ui.Fatal("Failed to save manifest: %v", err)
				}
				// Update peer sharing index
				if err := peer.RebuildIndex(); err != nil {
					ui.PrintError("Failed to update peer index: %v", err)
				}
				modelName := hf.FormatModelName(user, repo, selectedQuant.Name)
				fmt.Printf("Pulled %s\n", modelName)
				return
			}
		}

		// Pull the model from HuggingFace
		result, err := pullModelWithProgress(client, user, repo, selectedQuant)
		if err != nil {
			ui.Fatal("%v", err)
		}

		// Update peer sharing index
		if err := peer.RebuildIndex(); err != nil {
			ui.PrintError("Failed to update peer index: %v", err)
		}

		modelName := hf.FormatModelName(user, repo, selectedQuant.Name)
		if result.IsVision {
			fmt.Printf("Pulled %s (vision model)\n", modelName)
		} else {
			fmt.Printf("Pulled %s\n", modelName)
		}
	},
}

// pullModelWithProgress wraps hf.PullModel with progress bar display.
func pullModelWithProgress(client *hf.Client, user, repo string, quant hf.Quantization) (*hf.PullResult, error) {
	// Get manifest info for display (also returns manifest to pass to PullModel)
	info, manifest, manifestJSON, err := hf.GetManifestInfo(client, user, repo, quant)
	if err != nil {
		return nil, err
	}

	modelName := ui.Keyword(hf.FormatModelName(user, repo, quant.Name))
	if info.IsVision {
		fmt.Printf("Pulling %s (%s + %s mmproj)\n",
			modelName,
			ui.FormatBytes(info.GGUFSize),
			ui.FormatBytes(info.MMProjSize))
	} else {
		fmt.Printf("Pulling %s (%s)\n", modelName, ui.FormatBytes(info.GGUFSize))
	}

	opts := &hf.PullOptions{
		Manifest:     manifest,
		ManifestJSON: manifestJSON,
	}

	return hf.PullModelWithProgressFactory(client, user, repo, quant, opts, newProgressBar)
}

// newProgressBar creates a new progress bar that implements hf.ProgressDisplay.
func newProgressBar() hf.ProgressDisplay {
	return ui.NewProgressBar()
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
		fmt.Println("  • Use 'lleme search <query>' to find models")
	} else {
		fmt.Printf("%s %v\n", ui.ErrorMsg("Error:"), err)
	}
}

// peerMatch holds the result of a successful peer hash lookup.
type peerMatch struct {
	peer   *peer.Peer
	client *peer.Client
	size   int64
}

// tryPullFromPeer attempts to download a file from a peer using its SHA256 hash.
// The hash comes from the HuggingFace manifest (trusted source).
// Returns true if the file was successfully downloaded from a peer.
func tryPullFromPeer(user, repo, quant, hash string) bool {
	// Quick mDNS discovery to find peers
	peers := discoverPeersQuick()
	if len(peers) == 0 {
		return false
	}

	// Query all peers in parallel for the hash
	resultCh := make(chan peerMatch, len(peers))

	for _, p := range peers {
		go func(p *peer.Peer) {
			client := peer.NewClient(p)
			size, hasFile := client.HasHash(hash)
			if hasFile {
				resultCh <- peerMatch{peer: p, client: client, size: size}
			}
		}(p)
	}

	// Wait for first successful result or timeout
	var found *peerMatch
	select {
	case match := <-resultCh:
		found = &match
	case <-time.After(5 * time.Second):
		return false
	}

	// Found the file on a peer - attempt download
	modelName := ui.Keyword(hf.FormatModelName(user, repo, quant))
	fmt.Printf("Pulling %s from peer %s (%s)\n", modelName, found.peer.Host, ui.FormatBytes(found.size))

	bar := ui.NewProgressBar()
	bar.Start("", found.size)

	destPath := hf.GetModelFilePath(user, repo, quant)

	// Ensure model directory exists
	modelDir := hf.GetModelPath(user, repo)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		bar.Stop()
		fmt.Printf("Failed to create model directory: %v\n", err)
		return false
	}

	err := found.client.DownloadHash(hash, destPath, func(downloaded, total int64) {
		bar.Update(downloaded, total)
	})

	if err != nil {
		bar.Stop()
		os.Remove(destPath)
		os.Remove(destPath + ".partial")
		fmt.Printf("Peer download failed: %v - falling back to HuggingFace\n", err)
		return false
	}

	// Verify the download against the HF-provided hash (not peer-provided)
	bar.Finish("Verifying")
	valid, err := peer.VerifyDownload(destPath, hash)
	if err != nil || !valid {
		os.Remove(destPath)
		fmt.Printf("Verification failed - falling back to HuggingFace\n")
		return false
	}

	bar.Finish("Downloaded from peer")
	return true
}

// discoverPeersQuick does a fast mDNS query to find peers.
func discoverPeersQuick() []*peer.Peer {
	var peers []*peer.Peer

	entriesCh := make(chan *peer.Peer, 10)

	// Use a goroutine to collect results
	done := make(chan struct{})
	go func() {
		for p := range entriesCh {
			peers = append(peers, p)
		}
		close(done)
	}()

	// Do the actual mDNS query
	peer.QuickDiscover(entriesCh)

	<-done
	return peers
}

func init() {
	rootCmd.AddCommand(pullCmd)
}
