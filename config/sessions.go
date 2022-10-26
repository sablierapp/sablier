package config

import "time"

type Sessions struct {
	DefaultDuration    time.Duration `mapstructure:"DEFAULT_DURATION" yaml:"defaultDuration" default:"5m"`
	ExpirationInterval time.Duration `mapstructure:"EXPIRATION_INTERVAL" yaml:"expirationIntrerval" default:"20s"`
}

func NewSessionsConfig() Sessions {
	return Sessions{
		DefaultDuration:    5 * time.Minute,
		ExpirationInterval: 20 * time.Second,
	}
}
