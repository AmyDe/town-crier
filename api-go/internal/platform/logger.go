package platform

import (
	"io"
	"log/slog"
)

// NewLogger returns a JSON slog logger at the given level. Production wiring
// passes os.Stdout; tests pass a buffer.
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
