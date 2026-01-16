package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nchapman/gollama/internal/ui"
)

type ModelMatch struct {
	FullRef string
	User    string
	Repo    string
	Quant   string
	Path    string
	Score   int
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func findModels() []string {
	modelsDir := filepath.Join(os.Getenv("HOME"), ".gollama", "models")
	var models []string

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return models
	}

	for _, userDir := range entries {
		if !userDir.IsDir() {
			continue
		}

		reposPath := filepath.Join(modelsDir, userDir.Name())
		repos, err := os.ReadDir(reposPath)
		if err != nil {
			continue
		}

		for _, repoDir := range repos {
			if !repoDir.IsDir() {
				continue
			}

			models = append(models, filepath.Join(userDir.Name(), repoDir.Name()))
		}
	}

	return models
}

func findMatchingModels(input string) ([]ModelMatch, []ModelMatch) {
	normalizedInput := strings.ToLower(input)
	allModels := findModels()

	var exactMatches, suffixMatches, containsMatches []ModelMatch

	for _, modelPath := range allModels {
		parts := strings.Split(modelPath, string(filepath.Separator))
		if len(parts) != 2 {
			continue
		}

		user, repo := parts[0], parts[1]
		fullRef := user + "/" + repo

		match := ModelMatch{
			FullRef: fullRef,
			User:    user,
			Repo:    repo,
			Path:    modelPath,
		}

		if strings.EqualFold(fullRef, input) {
			match.Score = 100
			exactMatches = append(exactMatches, match)
			continue
		}

		if strings.HasSuffix(strings.ToLower(fullRef), normalizedInput) {
			match.Score = 75
			suffixMatches = append(suffixMatches, match)
			continue
		}

		if strings.Contains(strings.ToLower(fullRef), normalizedInput) {
			match.Score = 50
			containsMatches = append(containsMatches, match)
		}
	}

	if len(exactMatches) > 0 {
		return exactMatches, nil
	}

	if len(suffixMatches) > 0 {
		return suffixMatches, nil
	}

	if len(containsMatches) > 0 {
		return containsMatches, nil
	}

	fuzzyMatches := fuzzyMatch(input, allModels)
	return fuzzyMatches[:min(5, len(fuzzyMatches))], fuzzyMatches
}

func fuzzyMatch(input string, models []string) []ModelMatch {
	input = strings.ToLower(input)
	words := strings.Fields(input)

	var matches []ModelMatch

	for _, modelPath := range models {
		lowerPath := strings.ToLower(modelPath)
		score := 0

		for _, word := range words {
			if strings.Contains(lowerPath, word) {
				score += 10
			}
		}

		if score > 0 {
			parts := strings.Split(modelPath, string(filepath.Separator))
			if len(parts) == 2 {
				matches = append(matches, ModelMatch{
					FullRef: parts[0] + "/" + parts[1],
					User:    parts[0],
					Repo:    parts[1],
					Path:    modelPath,
					Score:   score,
				})
			}
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func printModelSuggestions(matches []ModelMatch) {
	fmt.Printf("\n%s\n", ui.Bold("Did you mean?"))
	for _, match := range matches {
		fmt.Printf("  • %s/%s\n", ui.Value(match.User), ui.Value(match.Repo))
	}
}
