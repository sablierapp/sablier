package config

type Server struct {
	Port     int    `mapstructure:"PORT" yaml:"port" default:"10000"`
	BasePath string `mapstructure:"BASE_PATH" yaml:"basePath" default:"/"`
}

func NewServerConfig() Server {
	return Server{
		Port:     10000,
		BasePath: "/",
	}
}
