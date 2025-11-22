package config

type Tracing struct {
	Enabled  bool   `mapstructure:"ENABLED" yaml:"enabled,omitempty" default:"false"`
	Endpoint string `mapstructure:"ENDPOINT" yaml:"endpoint,omitempty" default:"localhost:4317"`
}

func NewTracingConfig() Tracing {
	return Tracing{
		Enabled:  false,
		Endpoint: "localhost:4317",
	}
}
