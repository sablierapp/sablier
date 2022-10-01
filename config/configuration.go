package config

type Config struct {
	Server   Server
	Storage  Storage
	Provider Provider
}

func NewConfig() Config {
	return Config{
		Server:   NewServerConfig(),
		Storage:  NewStorageConfig(),
		Provider: NewProviderConfig(),
	}
}
