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
	Name string
	File string
	Size int64
}

func ParseQuantization(filename string) string {
	matches := quantPattern.FindStringSubmatch(filename)
	if len(matches) < 2 {
		return ""
	}

	quant := strings.ToUpper(matches[1])
	quant = strings.ReplaceAll(quant, "-", "_")

	normalizations := map[string]string{
		"F16": "FP16",
		"F32": "FP32",
		"I8":  "Q8_0",
		"I4":  "Q4_0",
	}

	if normalized, ok := normalizations[quant]; ok {
		return normalized
	}

	return quant
}

func ExtractQuantizations(files []FileTree) []Quantization {
	var quants []Quantization

	for _, file := range files {
		if !strings.HasSuffix(file.Path, ".gguf") {
			continue
		}

		quant := ParseQuantization(file.Path)
		if quant == "" {
			continue
		}

		quants = append(quants, Quantization{
			Name: quant,
			File: file.Path,
			Size: file.Size,
		})
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
		if strings.EqualFold(q.Name, name) {
			return q, true
		}
	}
	return Quantization{}, false
}

func IsGGUFFile(filename string) bool {
	return strings.HasSuffix(filename, ".gguf")
}
