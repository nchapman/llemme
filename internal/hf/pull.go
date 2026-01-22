package hf

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// PullResult contains the result of a model pull operation.
type PullResult struct {
	ModelPath  string
	MMProjPath string // Empty if not a vision model
	IsVision   bool
	TotalSize  int64
	GGUFSize   int64
	MMProjSize int64
}

// PullProgress is called during download and verification phases.
type PullProgress struct {
	Phase   string // "download" or "verify"
	Current int64
	Total   int64
}

// ManifestInfo contains size information about a model from its manifest.
type ManifestInfo struct {
	GGUFSize   int64
	MMProjSize int64
	TotalSize  int64
	IsVision   bool
}

// GetManifestInfo fetches the manifest and returns size information for display.
// Returns the manifest data so it can be passed to PullModel to avoid re-fetching.
func GetManifestInfo(client *Client, user, repo string, quant Quantization) (*ManifestInfo, *Manifest, []byte, error) {
	manifest, manifestJSON, err := client.GetManifest(user, repo, quant.Tag)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	if manifest.GGUFFile == nil {
		return nil, nil, nil, fmt.Errorf("manifest does not contain a GGUF file")
	}

	info := &ManifestInfo{
		GGUFSize:  manifest.GGUFFile.Size,
		TotalSize: manifest.GGUFFile.Size,
		IsVision:  manifest.MMProjFile != nil,
	}
	if info.IsVision {
		info.MMProjSize = manifest.MMProjFile.Size
		info.TotalSize += info.MMProjSize
	}

	return info, manifest, manifestJSON, nil
}

// PullOptions configures a model pull operation.
type PullOptions struct {
	// Pre-fetched manifest to avoid duplicate API calls.
	// If nil, PullModel will fetch it.
	Manifest     *Manifest
	ManifestJSON []byte
}

// PullModel downloads a model from HuggingFace using the manifest API.
// It handles downloading the GGUF file, optional mmproj for vision models,
// hash verification, and saving the manifest for future reference.
func PullModel(client *Client, user, repo string, quant Quantization, opts *PullOptions, progress func(PullProgress)) (*PullResult, error) {
	var manifest *Manifest
	var manifestJSON []byte
	var err error

	// Use pre-fetched manifest if provided, otherwise fetch it
	if opts != nil && opts.Manifest != nil {
		manifest = opts.Manifest
		manifestJSON = opts.ManifestJSON
	} else {
		manifest, manifestJSON, err = client.GetManifest(user, repo, quant.Tag)
		if err != nil {
			return nil, fmt.Errorf("failed to get manifest: %w", err)
		}
	}

	if manifest.GGUFFile == nil {
		return nil, fmt.Errorf("manifest does not contain a GGUF file")
	}

	// Calculate total download size
	result := &PullResult{
		GGUFSize:  manifest.GGUFFile.Size,
		TotalSize: manifest.GGUFFile.Size,
		IsVision:  manifest.MMProjFile != nil,
	}
	if result.IsVision {
		result.MMProjSize = manifest.MMProjFile.Size
		result.TotalSize += result.MMProjSize
	}

	// Create model directory
	modelDir := GetModelPath(user, repo)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create model directory: %w", err)
	}

	result.ModelPath = GetModelFilePath(user, repo, quant.Name)

	// Download main model
	downloaded := int64(0)
	downloaderWithProgress := NewDownloaderWithProgress(client, func(current, total int64, speed float64, eta time.Duration) {
		if progress != nil {
			progress(PullProgress{
				Phase:   "download",
				Current: downloaded + current,
				Total:   result.TotalSize,
			})
		}
	})

	_, err = downloaderWithProgress.DownloadModel(user, repo, "main", manifest.GGUFFile.RFilename, result.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to download model: %w", err)
	}
	downloaded += manifest.GGUFFile.Size

	// Download mmproj if present (vision model)
	if result.IsVision {
		result.MMProjPath = GetMMProjFilePath(user, repo, quant.Name)
		_, err = downloaderWithProgress.DownloadModel(user, repo, "main", manifest.MMProjFile.RFilename, result.MMProjPath)
		if err != nil {
			return nil, fmt.Errorf("failed to download mmproj: %w", err)
		}
	}

	// Verify downloaded files against manifest hashes
	if manifest.GGUFFile.LFS != nil {
		verified := int64(0)

		hash, err := CalculateSHA256WithProgress(result.ModelPath, func(processed, total int64) {
			if progress != nil {
				progress(PullProgress{
					Phase:   "verify",
					Current: verified + processed,
					Total:   result.TotalSize,
				})
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to verify model: %w", err)
		}
		if hash != manifest.GGUFFile.LFS.SHA256 {
			os.Remove(result.ModelPath)
			return nil, fmt.Errorf("model verification failed: hash mismatch")
		}
		verified += manifest.GGUFFile.Size

		if result.IsVision && manifest.MMProjFile.LFS != nil {
			hash, err := CalculateSHA256WithProgress(result.MMProjPath, func(processed, total int64) {
				if progress != nil {
					progress(PullProgress{
						Phase:   "verify",
						Current: verified + processed,
						Total:   result.TotalSize,
					})
				}
			})
			if err != nil {
				return nil, fmt.Errorf("failed to verify mmproj: %w", err)
			}
			if hash != manifest.MMProjFile.LFS.SHA256 {
				os.Remove(result.MMProjPath)
				return nil, fmt.Errorf("mmproj verification failed: hash mismatch")
			}
		}
	}

	// Save manifest for offline reference and verification
	manifestPath := GetManifestFilePath(user, repo, quant.Name)
	if err := os.WriteFile(manifestPath, manifestJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	return result, nil
}

// CheckForUpdates checks if a local model is up to date with the remote manifest.
// Returns (up-to-date, should-save-manifest, manifest, manifestJSON, error).
func CheckForUpdates(client *Client, user, repo string, quant Quantization) (bool, bool, *Manifest, []byte, error) {
	manifest, manifestJSON, err := client.GetManifest(user, repo, quant.Tag)
	if err != nil {
		return false, false, nil, nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	if manifest.GGUFFile == nil {
		return false, false, nil, nil, fmt.Errorf("manifest does not contain a GGUF file")
	}

	upToDate, saveManifest := isUpToDate(user, repo, quant.Name, manifest)
	return upToDate, saveManifest, manifest, manifestJSON, nil
}

// isUpToDate checks if local files match the remote manifest by comparing sha256 hashes.
// Returns (up-to-date, should-save-manifest).
func isUpToDate(user, repo, quant string, remote *Manifest) (bool, bool) {
	manifestPath := GetManifestFilePath(user, repo, quant)
	modelPath := GetModelFilePath(user, repo, quant)

	// Load saved manifest
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		// No local manifest - check if this is a legacy download we can upgrade
		modelInfo, statErr := os.Stat(modelPath)
		if statErr == nil && modelInfo.Size() == remote.GGUFFile.Size {
			// Model exists with matching size - likely a legacy download
			// Check if vision model needs mmproj
			if remote.MMProjFile != nil {
				mmprojPath := GetMMProjFilePath(user, repo, quant)
				if _, err := os.Stat(mmprojPath); err != nil {
					return false, false // Need to download mmproj
				}
			}
			// Save manifest for this legacy model and consider it up to date
			return true, true
		}
		return false, false
	}

	var local Manifest
	if err := parseManifest(manifestData, &local); err != nil {
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
		mmprojPath := GetMMProjFilePath(user, repo, quant)
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
func hashesMatch(local, remote *ManifestFile) bool {
	if local == nil || remote == nil {
		return local == nil && remote == nil
	}
	if local.LFS == nil || remote.LFS == nil {
		// No hash info, fall back to size comparison
		return local.Size == remote.Size
	}
	return local.LFS.SHA256 == remote.LFS.SHA256
}

// parseManifest unmarshals manifest JSON data.
func parseManifest(data []byte, manifest *Manifest) error {
	return json.Unmarshal(data, manifest)
}

// ProgressDisplay handles progress bar display for pull operations.
type ProgressDisplay interface {
	Start(label string, total int64)
	Update(current int64)
	Finish(label string)
	Stop()
}

// ProgressDisplayFactory creates new progress displays.
type ProgressDisplayFactory func() ProgressDisplay

// PullModelWithProgress downloads a model with progress bar display.
// Uses the provided factory to create progress bars for download and verify phases.
func PullModelWithProgress(client *Client, user, repo string, quant Quantization, opts *PullOptions) (*PullResult, error) {
	return PullModelWithProgressFactory(client, user, repo, quant, opts, nil)
}

// PullModelWithProgressFactory downloads a model with customizable progress display.
// If factory is nil, progress is not displayed.
func PullModelWithProgressFactory(client *Client, user, repo string, quant Quantization, opts *PullOptions, factory ProgressDisplayFactory) (*PullResult, error) {
	var progress ProgressDisplay
	var currentPhase string

	result, err := PullModel(client, user, repo, quant, opts, func(p PullProgress) {
		if factory == nil {
			return
		}
		if p.Phase != currentPhase {
			if progress != nil {
				if currentPhase == "download" {
					progress.Finish("Downloaded")
				} else {
					progress.Finish("Verified")
				}
			}
			currentPhase = p.Phase
			progress = factory()
			if p.Phase == "download" {
				progress.Start("", p.Total)
			} else {
				progress.Start("Verifying", p.Total)
			}
		}
		if progress != nil {
			progress.Update(p.Current)
		}
	})

	if progress != nil {
		if err != nil {
			progress.Stop()
		} else if currentPhase == "download" {
			progress.Finish("Downloaded")
		} else {
			progress.Finish("Verified")
		}
	}

	return result, err
}
