package logger

import (
	"log/slog"
	"os"
)

// Log is the global structured logger.
var Log *slog.Logger

// Init initializes the global logger based on debug and format flags.
func Init(debug bool, jsonOutput bool) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	if debug {
		opts.Level = slog.LevelDebug
	}

	var handler slog.Handler
	if jsonOutput {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	Log = slog.New(handler)
	slog.SetDefault(Log)
}
