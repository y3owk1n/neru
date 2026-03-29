package config

// Provider exposes the current configuration snapshot.
type Provider interface {
	Get() *Config
}
