package log

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// New builds a zerolog logger with the given level string (debug, info, warn, error).
func New(level string) *zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	lvl := parseLevel(level)
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	logger := zerolog.New(output).Level(lvl).With().Timestamp().Logger()
	return &logger
}

func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
