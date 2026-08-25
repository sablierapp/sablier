package sabliercmd

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/tracing"
)

func setupLogger(config config.Logging) *slog.Logger {
	return newLogger(os.Stderr, config)
}

// newLogger builds the application logger writing to w. It is separate from
// setupLogger so that tests can assert on what actually reaches the output.
func newLogger(w io.Writer, config config.Logging) *slog.Logger {
	inner := tint.NewHandler(w, &tint.Options{
		Level:      parseLogLevel(config.Level),
		TimeFormat: time.Kitchen,
		AddSource:  true,
		NoColor:    noColor(),
	})
	// OTelHandler is a zero-cost pass-through when no span is active.
	logger := slog.New(tracing.NewOTelHandler(inner))

	return logger
}

// noColor reports whether ANSI escape sequences must be left out of the log
// output.
//
// It implements https://no-color.org: color is suppressed when NO_COLOR is
// present and set to anything other than the empty string, whatever that value
// is. NO_COLOR=0 therefore disables color too, as the specification requires.
// os.Getenv cannot tell an unset variable from an empty one, but both mean
// "keep color", so the distinction does not matter here.
func noColor() bool {
	return os.Getenv("NO_COLOR") != ""
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
