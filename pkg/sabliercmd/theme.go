package sabliercmd

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/theme"
)

func setupTheme(ctx context.Context, conf config.Config, logger *slog.Logger) (*theme.Themes, error) {
	path := conf.Strategy.Dynamic.CustomThemesPath
	if path == "" {
		logger.DebugContext(ctx, "loading themes without custom theme path", slog.String("reason", "--strategy.dynamic.custom-themes-path is empty"))
		return theme.New(logger)
	}

	logger.DebugContext(ctx, "loading themes from custom theme path", slog.String("path", path))
	t, err := theme.NewWithCustomThemesFromPath(path, logger)
	if err == nil {
		return t, nil
	}

	// The custom themes path now defaults to /etc/sablier/themes, which nothing creates:
	// an absent directory is the normal case for every deployment that does not mount
	// custom themes, so it must not be reported as a problem. Anything else (permissions,
	// an unparsable template) is a real misconfiguration and stays a warning.
	if errors.Is(err, fs.ErrNotExist) {
		logger.DebugContext(ctx, "loading themes without custom theme path", slog.String("reason", "custom themes path does not exist"), slog.String("path", path))
	} else {
		logger.WarnContext(ctx, "loading themes without custom theme path", slog.String("reason", "failed to load custom themes"), slog.String("path", path), slog.String("error", err.Error()))
	}

	return theme.New(logger)
}
