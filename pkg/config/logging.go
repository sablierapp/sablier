package config

import (
	"log/slog"
	"strings"
)

type Logging struct {
	Level string `mapstructure:"LEVEL" yaml:"level" default:"info"`
}

func NewLoggingConfig() Logging {
	return Logging{
		Level: strings.ToLower(slog.LevelInfo.String()),
	}
}
