package config

import "sync"

var (
	globalConfig *Config
	globalMu     sync.RWMutex
)

// SetGlobal sets the global configuration instance for application-wide access.
// This function is thread-safe.
func SetGlobal(cfg *Config) {
	globalMu.Lock()
	defer globalMu.Unlock()

	globalConfig = cfg
}

// Global returns the global configuration instance.
// This function is thread-safe.
func Global() *Config {
	globalMu.RLock()
	defer globalMu.RUnlock()

	return globalConfig
}
