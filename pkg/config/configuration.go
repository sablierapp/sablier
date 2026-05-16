package config

type Config struct {
	Server   Server
	Storage  Storage
	Provider Provider
	Sessions Sessions
	Logging  Logging
	Strategy Strategy
	Webhooks Webhooks
}

func NewConfig() Config {
	return Config{
		Server:   NewServerConfig(),
		Storage:  NewStorageConfig(),
		Provider: NewProviderConfig(),
		Sessions: NewSessionsConfig(),
		Logging:  NewLoggingConfig(),
		Strategy: NewStrategyConfig(),
		Webhooks: NewWebhooksConfig(),
	}
}
