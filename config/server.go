package config

type Server struct {
	Port     int    `mapstructure:"PORT" yaml:"port" default:"10000"`
	BasePath string `mapstructure:"BASEPATH" yaml:"basePath" default:"/"`
}

func NewServerConfig() Server {
	return Server{
		Port:     10000,
		BasePath: "/",
	}
}
