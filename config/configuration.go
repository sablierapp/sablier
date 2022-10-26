package config

type Config struct {
	Server   Server
	Storage  Storage
	Provider Provider
	Sessions Sessions
}

func NewConfig() Config {
	return Config{
		Server:   NewServerConfig(),
		Storage:  NewStorageConfig(),
		Provider: NewProviderConfig(),
		Sessions: NewSessionsConfig(),
	}
}
