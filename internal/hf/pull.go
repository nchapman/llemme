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

	// Check if this is a split file by examining the manifest filename
	splitInfo := ParseSplitFilename(manifest.GGUFFile.RFilename)
	if splitInfo != nil && splitInfo.SplitNo != 0 {
		return nil, fmt.Errorf("manifest references split %d, expected first split", splitInfo.SplitNo+1)
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

	// If split file, fetch all split file info from HuggingFace (includes LFS hashes)
	if splitInfo != nil {
		splitFiles, err := fetchSplitFileInfo(client, user, repo, splitInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch split file info: %w", err)
		}
		manifest.SplitFiles = splitFiles

		// Calculate total size from all splits
		totalGGUFSize := manifest.GGUFFile.Size
		for _, sf := range splitFiles {
			totalGGUFSize += sf.Size
		}
		result.GGUFSize = totalGGUFSize
		result.TotalSize = totalGGUFSize
		if result.IsVision {
			result.TotalSize += result.MMProjSize
		}
	}

	// Create model directory
	modelDir := GetModelPath(user, repo)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create model directory: %w", err)
	}

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

	if splitInfo != nil {
		// Split file - download all parts to split directory
		result.ModelPath, err = handleSplitDownload(client, user, repo, quant, splitInfo, &downloaded, result, progress)
		if err != nil {
			splitDir := GetSplitModelDir(user, repo, quant.Name)
			os.RemoveAll(splitDir)
			return nil, err
		}
	} else {
		// Single file - download directly to final location
		result.ModelPath = GetModelFilePath(user, repo, quant.Name)
		_, err = downloaderWithProgress.DownloadModel(user, repo, "main", manifest.GGUFFile.RFilename, result.ModelPath)
		if err != nil {
			return nil, fmt.Errorf("failed to download model: %w", err)
		}
		downloaded += manifest.GGUFFile.Size
	}

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

		if splitInfo == nil {
			// Single file verification
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
		} else {
			// Split file verification - verify each split
			splitDir := GetSplitModelDir(user, repo, quant.Name)

			// Verify first split
			firstSplitPath := filepath.Join(splitDir, filepath.Base(manifest.GGUFFile.RFilename))
			hash, err := CalculateSHA256WithProgress(firstSplitPath, func(processed, total int64) {
				if progress != nil {
					progress(PullProgress{
						Phase:   "verify",
						Current: verified + processed,
						Total:   result.TotalSize,
					})
				}
			})
			if err != nil {
				return nil, fmt.Errorf("failed to verify split 1: %w", err)
			}
			if hash != manifest.GGUFFile.LFS.SHA256 {
				os.RemoveAll(splitDir)
				return nil, fmt.Errorf("split 1 verification failed: hash mismatch")
			}
			verified += manifest.GGUFFile.Size

			// Verify remaining splits
			for i, sf := range manifest.SplitFiles {
				if sf.LFS == nil {
					continue // Skip if no hash info
				}
				splitPath := filepath.Join(splitDir, filepath.Base(sf.RFilename))
				hash, err := CalculateSHA256WithProgress(splitPath, func(processed, total int64) {
					if progress != nil {
						progress(PullProgress{
							Phase:   "verify",
							Current: verified + processed,
							Total:   result.TotalSize,
						})
					}
				})
				if err != nil {
					return nil, fmt.Errorf("failed to verify split %d: %w", i+2, err)
				}
				if hash != sf.LFS.SHA256 {
					os.RemoveAll(splitDir)
					return nil, fmt.Errorf("split %d verification failed: hash mismatch", i+2)
				}
				verified += sf.Size
			}
		}

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
	// Re-marshal to include SplitFiles if present
	var manifestData []byte
	if len(manifest.SplitFiles) > 0 {
		manifestData, err = json.Marshal(manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal manifest: %w", err)
		}
	} else {
		manifestData = manifestJSON
	}
	manifestPath := GetManifestFilePath(user, repo, quant.Name)
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to save manifest: %w", err)
	}

	return result, nil
}

// handleSplitDownload downloads all split files and organizes them.
// Downloads sequentially with progress tracking. Returns the path to the first split file.
func handleSplitDownload(client *Client, user, repo string, quant Quantization, splitInfo *SplitInfo, downloaded *int64, result *PullResult, progress func(PullProgress)) (string, error) {
	modelDir := GetModelPath(user, repo)

	// Create quant subdirectory for split files
	splitDir := filepath.Join(modelDir, quant.Name)
	if err := os.MkdirAll(splitDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create split directory: %w", err)
	}

	var firstSplitPath string

	// Download all splits sequentially with progress tracking
	for i := 0; i < splitInfo.SplitCount; i++ {
		splitFilename := SplitPath(splitInfo.Prefix, i, splitInfo.SplitCount)
		localFilename := filepath.Base(splitFilename)
		localPath := filepath.Join(splitDir, localFilename)

		if i == 0 {
			firstSplitPath = localPath
		}

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
			return "", fmt.Errorf("failed to download split %d: %w", i+1, err)
		}
		*downloaded += dlResult.Total
	}

	return firstSplitPath, nil
}

// fetchSplitFileInfo fetches LFS metadata for all split files (except the first, which is in the manifest).
// Returns ManifestFile entries for splits 2 through N.
func fetchSplitFileInfo(client *Client, user, repo string, splitInfo *SplitInfo) ([]*ManifestFile, error) {
	if splitInfo.SplitCount <= 1 {
		return nil, nil
	}

	// Get the directory path from the split prefix (e.g., "Q4_K_M/model-Q4_K_M" -> "Q4_K_M")
	dirPath := filepath.Dir(splitInfo.Prefix)
	if dirPath == "." {
		dirPath = ""
	}

	// Fetch file listing from HuggingFace
	files, err := client.ListFilesInPath(user, repo, "main", dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list split files: %w", err)
	}

	// Build a map of filename -> file info for quick lookup
	fileMap := make(map[string]FileTree)
	for _, f := range files {
		fileMap[filepath.Base(f.Path)] = f
	}

	// Collect info for splits 2 through N
	var splitFiles []*ManifestFile
	for i := 1; i < splitInfo.SplitCount; i++ {
		splitPath := SplitPath(splitInfo.Prefix, i, splitInfo.SplitCount)
		splitName := filepath.Base(splitPath)

		ft, ok := fileMap[splitName]
		if !ok {
			return nil, fmt.Errorf("split file %s not found in repository", splitName)
		}

		mf := &ManifestFile{
			RFilename: splitPath,
			Size:      ft.Size,
		}

		// Extract LFS info if available (OID is the SHA256 hash)
		if ft.LFS.OID != "" {
			mf.LFS = &ManifestLFS{
				SHA256: ft.LFS.OID,
				Size:   ft.LFS.Size,
			}
		}

		splitFiles = append(splitFiles, mf)
	}

	return splitFiles, nil
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
	Update(current, total int64)
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
			progress.Update(p.Current, p.Total)
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
