package hf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
// split GGUF files, hash verification, and saving the manifest for future reference.
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

	// Download first file to temp location first (to check for splits)
	tempPath := filepath.Join(modelDir, ".download-"+quant.Name+".gguf")
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

	_, err = downloaderWithProgress.DownloadModel(user, repo, "main", manifest.GGUFFile.RFilename, tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to download model: %w", err)
	}
	downloaded += manifest.GGUFFile.Size

	// Check if this is a split file
	header, err := ReadGGUFHeader(tempPath)
	if err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to read GGUF header: %w", err)
	}

	if header.SplitCount > 1 {
		// This is a split file - download remaining parts sequentially
		result.ModelPath, err = handleSplitDownload(client, user, repo, quant, manifest, tempPath, header.SplitCount, &downloaded, result, progress)
		if err != nil {
			// Clean up split directory on failure (tempPath was moved inside handleSplitDownload)
			splitDir := GetSplitModelDir(user, repo, quant.Name)
			os.RemoveAll(splitDir)
			return nil, err
		}
	} else {
		// Single file - move to final location
		result.ModelPath = GetModelFilePath(user, repo, quant.Name)
		if err := os.Rename(tempPath, result.ModelPath); err != nil {
			os.Remove(tempPath)
			return nil, fmt.Errorf("failed to move model to final location: %w", err)
		}
	}

	// Download mmproj if present (vision model)
	if result.IsVision {
		result.MMProjPath = GetMMProjFilePath(user, repo, quant.Name)
		_, err = downloaderWithProgress.DownloadModel(user, repo, "main", manifest.MMProjFile.RFilename, result.MMProjPath)
		if err != nil {
			return nil, fmt.Errorf("failed to download mmproj: %w", err)
		}
	}

	// Verify downloaded files against manifest hashes (only for single files)
	// Split files are verified by llama.cpp when loading
	if manifest.GGUFFile.LFS != nil && header.SplitCount <= 1 {
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

// handleSplitDownload handles downloading remaining split files and organizing them.
// Downloads sequentially with progress tracking. Returns the path to the first split file.
func handleSplitDownload(client *Client, user, repo string, quant Quantization, manifest *Manifest, firstFilePath string, splitCount int, downloaded *int64, result *PullResult, progress func(PullProgress)) (string, error) {
	modelDir := GetModelPath(user, repo)

	// Create quant subdirectory for split files
	splitDir := filepath.Join(modelDir, quant.Name)
	if err := os.MkdirAll(splitDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create split directory: %w", err)
	}

	// Extract the original filename from the manifest's rfilename
	// e.g., "Q4_K_M/gpt-oss-120b-Q4_K_M-00001-of-00002.gguf"
	originalFilename := filepath.Base(manifest.GGUFFile.RFilename)

	// Move first file to the split directory with original name
	firstSplitPath := filepath.Join(splitDir, originalFilename)
	if err := os.Rename(firstFilePath, firstSplitPath); err != nil {
		return "", fmt.Errorf("failed to move first split: %w", err)
	}

	// Extract the URL prefix from the manifest's rfilename for remaining splits
	// The rfilename contains the subdirectory: "Q4_K_M/gpt-oss-120b-Q4_K_M-00001-of-00002.gguf"
	rfilename := manifest.GGUFFile.RFilename
	splitPrefix := SplitPrefix(rfilename, 0, splitCount)
	if splitPrefix == "" {
		return "", fmt.Errorf("could not extract split prefix from %s", rfilename)
	}

	// Get sizes of remaining splits via HEAD requests for accurate progress
	splitFilenames := make([]string, splitCount-1)
	for idx := 1; idx < splitCount; idx++ {
		splitFilenames[idx-1] = SplitPath(splitPrefix, idx, splitCount)
	}

	var remainingSize int64
	for _, filename := range splitFilenames {
		size, err := client.GetFileSize(user, repo, "main", filename)
		if err != nil {
			// Fall back to estimate if HEAD fails
			remainingSize = manifest.GGUFFile.Size * int64(splitCount-1)
			break
		}
		remainingSize += size
	}

	// Update total size with accurate value
	result.TotalSize = manifest.GGUFFile.Size + remainingSize
	result.GGUFSize = result.TotalSize
	if result.IsVision {
		result.TotalSize += result.MMProjSize
	}

	// Download remaining splits sequentially with progress tracking
	for idx, splitFilename := range splitFilenames {
		localFilename := filepath.Base(splitFilename)
		localPath := filepath.Join(splitDir, localFilename)

		downloader := NewDownloaderWithProgress(client, func(current, total int64, speed float64, eta time.Duration) {
			if progress != nil {
				progress(PullProgress{
					Phase:   "download",
					Current: *downloaded + current,
					Total:   result.TotalSize,
				})
			}
		})

		dlResult, err := downloader.DownloadModel(user, repo, "main", splitFilename, localPath)
		if err != nil {
			os.RemoveAll(splitDir)
			return "", fmt.Errorf("failed to download split %d: %w", idx+2, err)
		}
		*downloaded += dlResult.Total
	}

	return firstSplitPath, nil
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

	// Find the model file (single or split)
	modelPath := FindModelFile(user, repo, quant)

	// Load saved manifest
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		// No local manifest - check if this is a legacy download we can upgrade
		if modelPath != "" {
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
	if modelPath == "" {
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
