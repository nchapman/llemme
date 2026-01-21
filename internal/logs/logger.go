package logs

import (
	"io"
	"os"

	"github.com/charmbracelet/log"
)

var logger *log.Logger

// InitLogger initializes the global logger. If w is nil, logs to stderr.
func InitLogger(w io.Writer, verbose bool) {
	if w == nil {
		w = os.Stderr
	}
	logger = log.NewWithOptions(w, log.Options{
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
