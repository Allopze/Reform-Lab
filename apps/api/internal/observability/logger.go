package observability

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// LoggerAlias is an alias so other packages can reference the logger type.
type LoggerAlias = zerolog.Logger

// NewLogger creates a structured logger configured for the given level.
func NewLogger(level string) zerolog.Logger {
	var w io.Writer = os.Stdout

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	return zerolog.New(w).
		With().
		Timestamp().
		Logger().
		Level(lvl)
}
