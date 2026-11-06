//go:build linux

package linux

import (
	"sync"

	"github.com/y3owk1n/neru/internal/config"
)

// configProvider is a process-global runtime config source for the Linux
// injection backends, set once at daemon startup
// (internal/app/runtime_config_linux.go) and read by the smooth-scroll path.
// It is a separate slot from platform/linux's because the two packages sit on
// opposite sides of the adapter boundary and neither imports the other. A nil
// provider (tests, before wiring) means the direct, unanimated scroll.
var (
	configProviderMu sync.RWMutex
	configProvider   config.Provider
)

// SetConfigProvider updates the runtime config provider used by the Linux
// scroll path.
func SetConfigProvider(provider config.Provider) {
	configProviderMu.Lock()
	configProvider = provider
	configProviderMu.Unlock()
}

// currentLinuxConfig returns the latest config snapshot, or nil when no
// provider has been wired yet.
func currentLinuxConfig() *config.Config {
	configProviderMu.RLock()

	provider := configProvider

	configProviderMu.RUnlock()

	if provider == nil {
		return nil
	}

	return provider.Get()
}
