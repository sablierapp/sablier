package config

type Server struct {
	Port     int           `mapstructure:"PORT" yaml:"port" default:"10000"`
	BasePath string        `mapstructure:"BASE_PATH" yaml:"basePath" default:"/"`
	Metrics  MetricsConfig `mapstructure:"METRICS" yaml:"metrics"`
}

type MetricsConfig struct {
	Enabled bool `mapstructure:"ENABLED" yaml:"enabled" default:"false"`
}

func NewServerConfig() Server {
	return Server{
		Port:     10000,
		BasePath: "/",
		Metrics:  MetricsConfig{Enabled: false},
	}
}
