package status

import (
	"log/slog"
	"os"
)

// NewLogger creates a structured logger with the default handler.
// Uses JSON format for machine readability in production, text for development.
func NewLogger(jsonFormat bool) *slog.Logger {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
