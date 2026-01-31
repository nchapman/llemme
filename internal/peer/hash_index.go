package peer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nchapman/lleme/internal/config"
	"github.com/nchapman/lleme/internal/hf"
	"github.com/nchapman/lleme/internal/logs"
)

// IndexFilePath returns the path to the persisted hash index file.
func IndexFilePath() string {
	return filepath.Join(config.BaseDir(), "hash_index.json")
}

// HashIndex maps SHA256 hashes to local file paths for peer sharing.
type HashIndex struct {
	mu    sync.RWMutex
	index map[string]string // sha256 -> model file path
}

// NewHashIndex creates a new hash index.
func NewHashIndex() *HashIndex {
	return &HashIndex{
		index: make(map[string]string),
	}
}

// Load reads the index from the persisted file.
func (h *HashIndex) Load() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := os.ReadFile(IndexFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			h.index = make(map[string]string)
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &h.index)
}

// Lookup returns the file path for a given SHA256 hash.
// Returns empty string if not found.
func (h *HashIndex) Lookup(hash string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.index[hash]
}

// Count returns the number of indexed files.
func (h *HashIndex) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.index)
}

// RebuildIndex scans all manifest files and saves the index to disk.
// Call this after pulling or deleting models.
func RebuildIndex() error {
	index := make(map[string]string)
	modelsDir := config.ModelsPath()

	// Use WalkDir for better performance (avoids stat on every file)
	err := filepath.WalkDir(modelsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		// Fast check before any allocations
		name := d.Name()
		if len(name) < 15 || name[len(name)-14:] != "-manifest.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var manifest hf.Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil
		}

		// Extract user/repo/quant from manifest path
		// Path format: ~/.lleme/models/{user}/{repo}/{quant}-manifest.json
		rel, err := filepath.Rel(modelsDir, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 3 {
			return nil
		}
		user, repo := parts[0], parts[1]
		quant := name[:len(name)-14] // Strip "-manifest.json"

		// Index main GGUF file (normalize hash to lowercase)
		if manifest.GGUFFile != nil && manifest.GGUFFile.LFS != nil && manifest.GGUFFile.LFS.SHA256 != "" {
			filePath := resolveFilePath(user, repo, quant, manifest.GGUFFile)
			if filePath != "" {
				index[strings.ToLower(manifest.GGUFFile.LFS.SHA256)] = filePath
			}
		}

		// Index additional split files
		for _, sf := range manifest.SplitFiles {
			if sf != nil && sf.LFS != nil && sf.LFS.SHA256 != "" {
				filePath := resolveFilePath(user, repo, quant, sf)
				if filePath != "" {
					index[strings.ToLower(sf.LFS.SHA256)] = filePath
				}
			}
		}

		// Index mmproj file for vision models
		if manifest.MMProjFile != nil && manifest.MMProjFile.LFS != nil && manifest.MMProjFile.LFS.SHA256 != "" {
			mmprojPath := hf.GetMMProjFilePath(user, repo, quant)
			if _, err := os.Stat(mmprojPath); err == nil {
				index[strings.ToLower(manifest.MMProjFile.LFS.SHA256)] = mmprojPath
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Save to file atomically (write to temp, then rename)
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}

	indexPath := IndexFilePath()
	tmpPath := indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, indexPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	logs.Debug("Hash index rebuilt", "entries", len(index))
	return nil
}

// resolveFilePath finds the actual file path for a manifest file entry.
func resolveFilePath(user, repo, quant string, mf *hf.ManifestFile) string {
	// Check if it's a split file
	splitInfo := hf.ParseSplitFilename(mf.RFilename)
	if splitInfo != nil {
		// For split files, return the specific split file path
		splitDir := hf.GetSplitModelDir(user, repo, quant)
		splitPath := filepath.Join(splitDir, filepath.Base(mf.RFilename))
		if _, err := os.Stat(splitPath); err == nil {
			return splitPath
		}
		return ""
	}

	// Single file
	modelPath := hf.GetModelFilePath(user, repo, quant)
	if _, err := os.Stat(modelPath); err == nil {
		return modelPath
	}
	return ""
}
