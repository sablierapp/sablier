package config

import log "github.com/sirupsen/logrus"

type Logging struct {
	Level string `mapstructure:"LEVEL" yaml:"level" default:"info"`
}

func NewLoggingLevel() Logging {
	return Logging{
		Level: log.InfoLevel.String(),
	}
}
