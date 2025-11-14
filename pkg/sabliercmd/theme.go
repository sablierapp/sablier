package sabliercmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/theme"
)

func setupTheme(ctx context.Context, conf config.Config, logger *slog.Logger) (*theme.Themes, error) {
	if conf.Strategy.Dynamic.CustomThemesPath != "" {
		logger.DebugContext(ctx, "loading themes from custom theme path", slog.String("path", conf.Strategy.Dynamic.CustomThemesPath))
		custom := os.DirFS(conf.Strategy.Dynamic.CustomThemesPath)
		t, err := theme.NewWithCustomThemes(custom, logger)
		if err != nil {
			return nil, err
		}
		return t, nil
	}
	logger.DebugContext(ctx, "loading themes without custom theme path", slog.String("reason", "--strategy.dynamic.custom-themes-path is empty"))
	t, err := theme.New(logger)
	if err != nil {
		return nil, err

	}
	return t, nil
}
