package config

import "time"

// DynamicStrategy holds configuration for the dynamic (waiting-page) strategy.
type DynamicStrategy struct {
	// CustomThemesPath is a directory from which Sablier loads custom waiting-page themes.
	// All .html files found recursively under this path are registered as named themes.
	// Leave empty to use only the built-in themes.
	// Env: SABLIER_STRATEGY_DYNAMIC_CUSTOM_THEMES_PATH
	// CLI: --strategy.dynamic.custom-themes-path
	// Default: "" (built-in themes only)
	// Since: v1.0.0
	CustomThemesPath string

	// ShowDetailsByDefault controls whether the waiting page shows per-instance
	// status details without requiring the caller to opt in.
	// Env: SABLIER_STRATEGY_DYNAMIC_SHOW_DETAILS_BY_DEFAULT
	// CLI: --strategy.dynamic.show-details-by-default
	// Default: true
	// Since: v1.0.0
	ShowDetailsByDefault bool

	// DefaultTheme is the name of the waiting-page theme used when the caller does not specify one.
	// Env: SABLIER_STRATEGY_DYNAMIC_DEFAULT_THEME
	// CLI: --strategy.dynamic.default-theme
	// Default: "hacker-terminal"
	// Since: v1.0.0
	DefaultTheme string

	// DefaultRefreshFrequency is how often the waiting page polls Sablier for an updated
	// readiness status when no frequency is specified by the caller.
	// Env: SABLIER_STRATEGY_DYNAMIC_DEFAULT_REFRESH_FREQUENCY
	// CLI: --strategy.dynamic.default-refresh-frequency
	// Default: 5s
	// Since: v1.0.0
	DefaultRefreshFrequency time.Duration
}

// BlockingStrategy holds configuration for the blocking strategy.
type BlockingStrategy struct {
	// DefaultTimeout is the maximum time the blocking strategy waits for all requested
	// instances to become ready before returning a timeout error.
	// Env: SABLIER_STRATEGY_BLOCKING_DEFAULT_TIMEOUT
	// CLI: --strategy.blocking.default-timeout
	// Default: 1m
	// Since: v1.0.0
	DefaultTimeout time.Duration

	// DefaultRefreshFrequency is how often the blocking strategy polls instance readiness
	// while waiting for workloads to start.
	// Env: SABLIER_STRATEGY_BLOCKING_DEFAULT_REFRESH_FREQUENCY
	// CLI: --strategy.blocking.default-refresh-frequency
	// Default: 5s
	// Since: v1.9.0
	DefaultRefreshFrequency time.Duration
}

type Strategy struct {
	Dynamic  DynamicStrategy
	Blocking BlockingStrategy
}

func NewStrategyConfig() Strategy {
	return Strategy{
		Dynamic:  newDynamicStrategy(),
		Blocking: newBlockingStrategy(),
	}
}

func newDynamicStrategy() DynamicStrategy {
	return DynamicStrategy{
		DefaultTheme:            "hacker-terminal",
		ShowDetailsByDefault:    true,
		DefaultRefreshFrequency: 5 * time.Second,
	}
}

func newBlockingStrategy() BlockingStrategy {
	return BlockingStrategy{
		DefaultTimeout:          1 * time.Minute,
		DefaultRefreshFrequency: 5 * time.Second,
	}
}
