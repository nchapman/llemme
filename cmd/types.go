package cmd

import "time"

// ModelInfo represents a locally downloaded model.
type ModelInfo struct {
	User     string
	Repo     string
	Quant    string
	Size     int64
	LastUsed time.Time
}
