package hf

import (
	"regexp"
	"sort"
	"strings"
)

var (
	quantPattern = regexp.MustCompile(`(?i)[._-](Q[0-9]+[^.]+|FP16|FP32|F16|F32|I[0-9]+)\.gguf`)
	quantOrder   = []string{
		"Q4_K_M",
		"Q4_K_S",
		"Q5_K_M",
		"Q5_K_S",
		"Q5_0",
		"Q5_1",
		"Q6_K",
		"Q8_0",
		"Q3_K_M",
		"Q3_K_S",
		"Q3_K_L",
		"Q2_K",
		"Q2_K_S",
		"Q2_K_L",
		"Q4_0",
		"Q4_1",
		"FP16",
		"F16",
		"FP32",
		"F32",
	}
)

type Quantization struct {
	Name string // Normalized name for display/comparison (e.g., "FP16", "Q4_K_M")
	Tag  string // Raw tag for HuggingFace API calls (e.g., "F16", "Q4_K_M")
	File string
	Size int64
}

// ParseQuantization extracts the quantization from a filename.
// Returns (normalized, raw) where normalized is for display/comparison and raw is for API calls.
// Returns ("", "") if no quantization found.
func ParseQuantization(filename string) (normalized string, raw string) {
	matches := quantPattern.FindStringSubmatch(filename)
	if len(matches) < 2 {
		return "", ""
	}

	raw = strings.ToUpper(matches[1])
	raw = strings.ReplaceAll(raw, "-", "_")

	normalizations := map[string]string{
		"F16": "FP16",
		"F32": "FP32",
		"I8":  "Q8_0",
		"I4":  "Q4_0",
	}

	if norm, ok := normalizations[raw]; ok {
		return norm, raw
	}

	return raw, raw
}

// quantDirPattern matches directory names that look like quantization names
var quantDirPattern = regexp.MustCompile(`^(?i)(Q[0-9]+[^/]*|FP16|FP32|F16|F32|I[0-9]+)$`)

func ExtractQuantizations(files []FileTree) []Quantization {
	var quants []Quantization
	seenQuants := make(map[string]bool)

	for _, file := range files {
		// Check for GGUF files
		if strings.HasSuffix(file.Path, ".gguf") {
			name, tag := ParseQuantization(file.Path)
			if name == "" {
				// GGUF file without quantization suffix - use "default"/"latest"
				name = "default"
				tag = "latest"
			}

			if seenQuants[name] {
				continue
			}
			seenQuants[name] = true

			quants = append(quants, Quantization{
				Name: name,
				Tag:  tag,
				File: file.Path,
				Size: file.Size,
			})
			continue
		}

		// Check for directories that look like quantization names
		// These contain split files or nested GGUF files
		if file.Type == "directory" && quantDirPattern.MatchString(file.Path) {
			// Normalize the directory name to a quantization name
			name := strings.ToUpper(file.Path)
			name = strings.ReplaceAll(name, "-", "_")

			// Apply normalizations
			normalizations := map[string]string{
				"F16": "FP16",
				"F32": "FP32",
				"I8":  "Q8_0",
				"I4":  "Q4_0",
			}
			normalized := name
			if norm, ok := normalizations[name]; ok {
				normalized = norm
			}

			if seenQuants[normalized] {
				continue
			}
			seenQuants[normalized] = true

			quants = append(quants, Quantization{
				Name: normalized,
				Tag:  file.Path, // Use original directory name as tag
				File: "",        // Will be resolved from manifest
				Size: 0,         // Size unknown until manifest fetch
			})
		}
	}

	return quants
}

func GetBestQuantization(quants []Quantization) string {
	if len(quants) == 0 {
		return ""
	}

	for _, preferred := range quantOrder {
		for _, q := range quants {
			if q.Name == preferred {
				return q.Name
			}
		}
	}

	return quants[0].Name
}

func SortQuantizations(quants []Quantization) []Quantization {
	sort.Slice(quants, func(i, j int) bool {
		orderI := getQuantOrder(quants[i].Name)
		orderJ := getQuantOrder(quants[j].Name)
		if orderI != orderJ {
			return orderI < orderJ
		}
		return quants[i].Name < quants[j].Name
	})

	return quants
}

func getQuantOrder(quant string) int {
	for i, q := range quantOrder {
		if q == quant {
			return i
		}
	}
	return len(quantOrder)
}

func FindQuantization(quants []Quantization, name string) (Quantization, bool) {
	for _, q := range quants {
		if strings.EqualFold(q.Name, name) || strings.EqualFold(q.Tag, name) {
			return q, true
		}
	}
	return Quantization{}, false
}

func IsGGUFFile(filename string) bool {
	return strings.HasSuffix(filename, ".gguf")
}

// FormatModelName returns a display name for the model, omitting the quant suffix for "default".
func FormatModelName(user, repo, quant string) string {
	if quant == "" || quant == "default" {
		return user + "/" + repo
	}
	return user + "/" + repo + ":" + quant
}
