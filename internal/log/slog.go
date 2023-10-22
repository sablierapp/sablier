package log

import (
	"log/slog"
	"os"

	"github.com/acouvreur/sablier/config"
)

func NewSlogLogger(config config.Logging) (Logger, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	return SlogLogger{logger: logger}, nil
}

type SlogLogger struct {
	logger *slog.Logger
}

type Logger interface {
	Debug(msg string, args ...any)
	Error(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}
