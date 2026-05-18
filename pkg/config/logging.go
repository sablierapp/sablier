package config

import (
	"log/slog"
	"strings"
)

// Logging holds the logging configuration.
type Logging struct {
	// Level sets the minimum log severity. Accepted values: debug, info, warn, error.
	// Env: SABLIER_LOGGING_LEVEL
	// CLI: --logging.level
	// Default: "info"
	Level string
}

func NewLoggingConfig() Logging {
	return Logging{
		Level: strings.ToLower(slog.LevelInfo.String()),
	}
}
