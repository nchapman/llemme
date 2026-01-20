package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nchapman/llemme/internal/config"
	"github.com/nchapman/llemme/internal/hf"
	"github.com/nchapman/llemme/internal/ui"
	"github.com/spf13/cobra"
)

var (
	rmForce      bool
	rmOlderThan  string
	rmLargerThan string
)

var removeCmd = &cobra.Command{
	Use:     "remove [pattern]",
	Aliases: []string{"rm"},
	Short:   "Remove downloaded models",
	GroupID: "model",
	Long: `Remove downloaded models by name, pattern, or filter.

Examples:
  llemme remove user/repo:quant       Remove specific model
  llemme remove user/repo             Remove all quants of a model
  llemme remove user/*                Remove all models from user
  llemme remove *                     Remove all models
  llemme remove --older-than 30d      Remove models unused for 30 days
  llemme remove --larger-than 10GB    Remove models larger than 10GB
  llemme remove user/* --older-than 7d  Combine pattern with filter`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pattern := ""
		if len(args) > 0 {
			pattern = args[0]
		}

		// Must have pattern or filters
		if pattern == "" && rmOlderThan == "" && rmLargerThan == "" {
			fmt.Printf("%s Specify a model pattern or use --older-than/--larger-than\n", ui.ErrorMsg("Error:"))
			fmt.Println()
			fmt.Println("Examples:")
			fmt.Println("  llemme remove user/repo:quant")
			fmt.Println("  llemme remove --older-than 30d")
			fmt.Println("  llemme remove --larger-than 10GB")
			os.Exit(1)
		}

		// Parse filters
		var olderThan time.Duration
		var largerThan int64
		var err error

		if rmOlderThan != "" {
			olderThan, err = parseDuration(rmOlderThan)
			if err != nil {
				fmt.Printf("%s Invalid --older-than value: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
		}

		if rmLargerThan != "" {
			largerThan, err = parseSize(rmLargerThan)
			if err != nil {
				fmt.Printf("%s Invalid --larger-than value: %v\n", ui.ErrorMsg("Error:"), err)
				os.Exit(1)
			}
		}

		// Default pattern to * when using filters
		if pattern == "" {
			pattern = "*"
		}

		// Find matching models
		models, err := findModels(pattern, olderThan, largerThan)
		if err != nil {
			fmt.Printf("%s %v\n", ui.ErrorMsg("Error:"), err)
			os.Exit(1)
		}

		if len(models) == 0 {
			fmt.Println("No models match the criteria")
			return
		}

		// Calculate total size
		var totalSize int64
		for _, m := range models {
			totalSize += m.Size
		}

		if !rmForce {
			fmt.Println("Models to remove:")
			fmt.Println()
			for _, m := range models {
				fmt.Printf("  %s/%s:%s (%s)\n", m.User, m.Repo, m.Quant, ui.FormatBytes(m.Size))
			}
			fmt.Println()

			if len(models) == 1 {
				m := models[0]
				fmt.Printf("Remove %s/%s:%s (%s)? [y/N] ", m.User, m.Repo, m.Quant, ui.FormatBytes(m.Size))
			} else {
				fmt.Printf("Remove %d model(s), %s total? [y/N] ", len(models), ui.FormatBytes(totalSize))
			}

			var response string
			if _, err := fmt.Scanln(&response); err != nil || (response != "y" && response != "Y") {
				fmt.Println(ui.Muted("Cancelled"))
				return
			}
		}

		// Remove models
		removed := 0
		var freedSize int64
		for _, m := range models {
			modelPath := hf.GetModelFilePath(m.User, m.Repo, m.Quant)
			if err := os.Remove(modelPath); err != nil {
				fmt.Printf("%s Failed to remove %s/%s:%s: %v\n", ui.ErrorMsg("Error:"), m.User, m.Repo, m.Quant, err)
				continue
			}
			freedSize += m.Size

			// Also remove associated files (manifest, mmproj, metadata)
			// These may not exist; errors are expected and safe to ignore
			os.Remove(hf.GetManifestFilePath(m.User, m.Repo, m.Quant))
			os.Remove(hf.GetMMProjFilePath(m.User, m.Repo, m.Quant))

			// Clean up empty directories
			modelDir := hf.GetModelPath(m.User, m.Repo)
			cleanEmptyDir(modelDir)
			userDir := filepath.Dir(modelDir)
			cleanEmptyDir(userDir)

			removed++
		}

		if removed == 1 {
			m := models[0]
			fmt.Printf("Removed %s/%s:%s\n", m.User, m.Repo, m.Quant)
		} else {
			fmt.Printf("Removed %d models, %s freed\n", removed, ui.FormatBytes(freedSize))
		}
	},
}

// findModels returns models matching the pattern and filters
func findModels(pattern string, olderThan time.Duration, largerThan int64) ([]ModelInfo, error) {
	return findModelsInDir(config.ModelsPath(), pattern, olderThan, largerThan)
}

// findModelsInDir is the testable version of findModels
func findModelsInDir(modelsDir, pattern string, olderThan time.Duration, largerThan int64) ([]ModelInfo, error) {
	var models []ModelInfo

	// Convert glob pattern to regex
	regexPattern := globToRegex(pattern)
	re, err := regexp.Compile("^" + regexPattern + "$")
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %s", pattern)
	}

	err = filepath.WalkDir(modelsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(d.Name()) != ".gguf" {
			return nil
		}

		relPath, err := filepath.Rel(modelsDir, path)
		if err != nil {
			return err
		}

		parts := strings.Split(relPath, string(filepath.Separator))
		if len(parts) < 3 {
			return nil
		}

		user := parts[0]
		repo := parts[1]
		quant := strings.TrimSuffix(d.Name(), ".gguf")

		// Check pattern match
		fullName := fmt.Sprintf("%s/%s:%s", user, repo, quant)
		repoName := fmt.Sprintf("%s/%s", user, repo)

		// Try matching full name, repo name, or repo/* pattern
		if !re.MatchString(fullName) && !re.MatchString(repoName) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Get last used time
		lastUsed := hf.GetLastUsed(user, repo, quant)
		if lastUsed.IsZero() {
			lastUsed = info.ModTime()
		}

		// Apply filters
		if olderThan > 0 {
			if time.Since(lastUsed) < olderThan {
				return nil
			}
		}

		if largerThan > 0 {
			if info.Size() < largerThan {
				return nil
			}
		}

		models = append(models, ModelInfo{
			User:     user,
			Repo:     repo,
			Quant:    quant,
			Size:     info.Size(),
			LastUsed: lastUsed,
		})

		return nil
	})

	return models, err
}

// globToRegex converts a glob pattern to a regex pattern
func globToRegex(glob string) string {
	result := regexp.QuoteMeta(glob)
	result = strings.ReplaceAll(result, `\*`, ".*")
	result = strings.ReplaceAll(result, `\?`, ".")
	return result
}

// parseDuration parses a duration string like "30d", "7d", "1w"
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]
	value, err := strconv.Atoi(valueStr)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	switch unit {
	case 'd':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case 'h':
		return time.Duration(value) * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit: %c (use d, w, or h)", unit)
	}
}

// parseSize parses a size string like "10GB", "500MB"
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid size: %s", s)
	}

	// Find where the number ends
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}

	valueStr := s[:i]
	unit := s[i:]

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size: %s", s)
	}

	switch unit {
	case "B":
		return int64(value), nil
	case "KB", "K":
		return int64(value * 1024), nil
	case "MB", "M":
		return int64(value * 1024 * 1024), nil
	case "GB", "G":
		return int64(value * 1024 * 1024 * 1024), nil
	case "TB", "T":
		return int64(value * 1024 * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("invalid size unit: %s (use B, KB, MB, GB, TB)", unit)
	}
}

// cleanEmptyDir removes a directory if it's empty
func cleanEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		os.Remove(dir)
	}
}

func init() {
	removeCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Skip confirmation prompt")
	removeCmd.Flags().StringVar(&rmOlderThan, "older-than", "", "Remove models not used in this duration (e.g., 24h, 7d, 4w)")
	removeCmd.Flags().StringVar(&rmLargerThan, "larger-than", "", "Remove models larger than this size (e.g., 500MB, 10GB)")
	rootCmd.AddCommand(removeCmd)
}
