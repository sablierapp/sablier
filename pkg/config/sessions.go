package config

import "time"

// Sessions holds the session lifecycle configuration.
type Sessions struct {
	// DefaultDuration is the session lifetime when no explicit duration is provided by the plugin or API caller.
	// Env: SABLIER_SESSIONS_DEFAULT_DURATION
	// CLI: --sessions.default-duration
	// Default: 5m
	DefaultDuration time.Duration

	// ExpirationInterval is how often Sablier checks for and stops expired sessions.
	// A longer interval reduces CPU overhead; align it with your shortest session duration
	// (e.g. if all sessions are ≥1 h, 5 m is a reasonable trade-off).
	// Env: SABLIER_SESSIONS_EXPIRATION_INTERVAL
	// CLI: --sessions.expiration-interval
	// Default: 20s
	ExpirationInterval time.Duration
}

func NewSessionsConfig() Sessions {
	return Sessions{
		DefaultDuration:    5 * time.Minute,
		ExpirationInterval: 20 * time.Second,
	}
}
