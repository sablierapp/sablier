package sabliercmd

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/sablierapp/sablier/pkg/config"
)

func setupLogger(config config.Logging) *slog.Logger {
	w := os.Stderr
	level := parseLogLevel(config.Level)
	// create a new logger
	logger := slog.New(tint.NewHandler(w, &tint.Options{
		Level:      level,
		TimeFormat: time.Kitchen,
		AddSource:  true,
	}))

	return logger
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case slog.LevelDebug.String():
		return slog.LevelDebug
	case slog.LevelInfo.String():
		return slog.LevelInfo
	case slog.LevelWarn.String():
		return slog.LevelWarn
	case slog.LevelError.String():
		return slog.LevelError
	default:
		slog.Warn("invalid log level, defaulting to info", slog.String("level", level))
		return slog.LevelInfo
	}
}
