package config

type Logging struct {
	Level string `mapstructure:"LEVEL" yaml:"level" default:"info"`
}

func NewLoggingConfig() Logging {
	return Logging{}
}
