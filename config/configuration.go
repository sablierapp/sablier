package config

type Config struct {
	Server   Server
	Storage  Storage
	Provider Provider
	Sessions Sessions
	Logging  Logging
}

func NewConfig() Config {
	return Config{
		Server:   NewServerConfig(),
		Storage:  NewStorageConfig(),
		Provider: NewProviderConfig(),
		Sessions: NewSessionsConfig(),
		Logging:  NewLoggingLevel(),
	}
}
