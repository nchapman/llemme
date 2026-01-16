package ui

import (
	"os"

	"github.com/charmbracelet/log"
)

var logger *log.Logger

func InitLogger(verbose bool) {
	logger = log.NewWithOptions(os.Stderr, log.Options{
		Level:           log.InfoLevel,
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	if verbose {
		logger.SetLevel(log.DebugLevel)
	}
}

func Debug(msg string, args ...any) {
	if logger != nil {
		logger.Debug(msg, args...)
	}
}

func Info(msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}

func Warn(msg string, args ...any) {
	if logger != nil {
		logger.Warn(msg, args...)
	}
}
