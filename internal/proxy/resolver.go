package proxy

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nchapman/lleme/internal/config"
)

// DownloadedModel represents a model that has been downloaded locally
type DownloadedModel struct {
	User      string
	Repo      string
	Quant     string
	FullName  string // "user/repo:quant"
	ModelPath string // Absolute path to .gguf file
}

// ModelResolver handles fuzzy matching of model names against downloaded models
type ModelResolver struct {
	modelsPath string
}

// NewModelResolver creates a new model resolver
func NewModelResolver() *ModelResolver {
	return &ModelResolver{
		modelsPath: config.ModelsPath(),
	}
}

// ListDownloadedModels returns all downloaded models
func (r *ModelResolver) ListDownloadedModels() ([]DownloadedModel, error) {
	var models []DownloadedModel

	err := filepath.WalkDir(r.modelsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Ext(d.Name()) != ".gguf" {
			return nil
		}

		relPath, err := filepath.Rel(r.modelsPath, path)
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

		models = append(models, DownloadedModel{
			User:      user,
			Repo:      repo,
			Quant:     quant,
			FullName:  fmt.Sprintf("%s/%s:%s", user, repo, quant),
			ModelPath: path,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return models, nil
}

// ResolveResult contains the result of a model resolution
type ResolveResult struct {
	Model       *DownloadedModel
	Matches     []DownloadedModel // All matching models (for ambiguous case)
	Suggestions []DownloadedModel // Fuzzy suggestions (for no match case)
}

// Resolve attempts to find a downloaded model matching the given query
// Returns:
// - Exact match: Model is set, Matches has 1 item
// - Ambiguous: Model is nil, Matches has multiple items
// - No match: Model is nil, Matches is empty, Suggestions may have items
func (r *ModelResolver) Resolve(query string) (*ResolveResult, error) {
	models, err := r.ListDownloadedModels()
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	if len(models) == 0 {
		return &ResolveResult{}, nil
	}

	// Normalize the query
	query = strings.ToLower(strings.TrimSpace(query))

	// Priority 1: Exact match (full name with quant)
	for i := range models {
		if strings.ToLower(models[i].FullName) == query {
			return &ResolveResult{
				Model:   &models[i],
				Matches: []DownloadedModel{models[i]},
			}, nil
		}
	}

	// Priority 2: Exact match on user/repo (without quant)
	// Returns all quants for that repo
	if !strings.Contains(query, ":") {
		var repoMatches []DownloadedModel
		for i := range models {
			userRepo := strings.ToLower(fmt.Sprintf("%s/%s", models[i].User, models[i].Repo))
			if userRepo == query {
				repoMatches = append(repoMatches, models[i])
			}
		}
		if len(repoMatches) == 1 {
			return &ResolveResult{
				Model:   &repoMatches[0],
				Matches: repoMatches,
			}, nil
		}
		if len(repoMatches) > 1 {
			// Multiple quants - pick the best one (Q4_K_M preferred)
			best := pickBestQuant(repoMatches)
			return &ResolveResult{
				Model:   best,
				Matches: repoMatches,
			}, nil
		}
	}

	// Priority 3: Suffix match (repo name or repo:quant)
	var suffixMatches []DownloadedModel
	for i := range models {
		// Match on "repo:quant" or "repo"
		repoQuant := strings.ToLower(fmt.Sprintf("%s:%s", models[i].Repo, models[i].Quant))
		repo := strings.ToLower(models[i].Repo)
		if repoQuant == query || repo == query {
			suffixMatches = append(suffixMatches, models[i])
		}
	}
	if len(suffixMatches) == 1 {
		return &ResolveResult{
			Model:   &suffixMatches[0],
			Matches: suffixMatches,
		}, nil
	}
	if len(suffixMatches) > 1 {
		// If all from same repo, pick best quant
		if allSameRepo(suffixMatches) {
			best := pickBestQuant(suffixMatches)
			return &ResolveResult{
				Model:   best,
				Matches: suffixMatches,
			}, nil
		}
		// Ambiguous - different repos
		return &ResolveResult{
			Matches: suffixMatches,
		}, nil
	}

	// Priority 4: Contains match (case-insensitive)
	var containsMatches []DownloadedModel
	for i := range models {
		fullLower := strings.ToLower(models[i].FullName)
		if strings.Contains(fullLower, query) {
			containsMatches = append(containsMatches, models[i])
		}
	}
	if len(containsMatches) == 1 {
		return &ResolveResult{
			Model:   &containsMatches[0],
			Matches: containsMatches,
		}, nil
	}
	if len(containsMatches) > 1 {
		// If all from same repo, pick best quant
		if allSameRepo(containsMatches) {
			best := pickBestQuant(containsMatches)
			return &ResolveResult{
				Model:   best,
				Matches: containsMatches,
			}, nil
		}
		// Ambiguous - different repos
		return &ResolveResult{
			Matches: containsMatches,
		}, nil
	}

	// No matches - try fuzzy suggestions
	suggestions := fuzzyMatch(query, models)
	return &ResolveResult{
		Suggestions: suggestions,
	}, nil
}

// allSameRepo checks if all models are from the same user/repo
func allSameRepo(models []DownloadedModel) bool {
	if len(models) == 0 {
		return true
	}
	first := fmt.Sprintf("%s/%s", models[0].User, models[0].Repo)
	for _, m := range models[1:] {
		if fmt.Sprintf("%s/%s", m.User, m.Repo) != first {
			return false
		}
	}
	return true
}

// quantPriority returns a priority score for quantization (lower is better)
var quantPriority = map[string]int{
	"Q4_K_M": 1,
	"Q4_K_S": 2,
	"Q5_K_M": 3,
	"Q5_K_S": 4,
	"Q5_0":   5,
	"Q5_1":   6,
	"Q6_K":   7,
	"Q8_0":   8,
	"Q3_K_M": 9,
	"Q3_K_S": 10,
	"Q3_K_L": 11,
	"Q2_K":   12,
	"Q4_0":   13,
	"Q4_1":   14,
	"FP16":   15,
	"F16":    16,
	"FP32":   17,
	"F32":    18,
}

// pickBestQuant returns the model with the best quantization
func pickBestQuant(models []DownloadedModel) *DownloadedModel {
	if len(models) == 0 {
		return nil
	}

	best := &models[0]
	bestPriority := getQuantPriority(best.Quant)

	for i := 1; i < len(models); i++ {
		p := getQuantPriority(models[i].Quant)
		if p < bestPriority {
			best = &models[i]
			bestPriority = p
		}
	}

	return best
}

func getQuantPriority(quant string) int {
	quant = strings.ToUpper(quant)
	if p, ok := quantPriority[quant]; ok {
		return p
	}
	return 100 // Unknown quants get low priority
}

// fuzzyMatch finds models with similar names (for typo suggestions)
func fuzzyMatch(query string, models []DownloadedModel) []DownloadedModel {
	type scored struct {
		model DownloadedModel
		score int
	}

	var results []scored

	for _, m := range models {
		// Calculate a simple edit distance score
		score := levenshtein(query, strings.ToLower(m.FullName))
		// Also check against just the repo name
		repoScore := levenshtein(query, strings.ToLower(m.Repo))
		if repoScore < score {
			score = repoScore
		}
		// Only include if reasonably close
		if score <= len(query)/2+3 {
			results = append(results, scored{m, score})
		}
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].score < results[j].score
	})

	// Return top 3
	var suggestions []DownloadedModel
	for i := 0; i < len(results) && i < 3; i++ {
		suggestions = append(suggestions, results[i].model)
	}

	return suggestions
}

// levenshtein calculates the edit distance between two strings
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create matrix
	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}
