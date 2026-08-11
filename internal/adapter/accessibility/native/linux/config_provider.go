//go:build linux

package linux

import (
	"sync"

	"github.com/y3owk1n/neru/internal/config"
)

// configProvider is a process-global runtime config source for the Linux
// injection backends, mirroring the slot platform/linux and platform/darwin
// already have. It is set once at daemon startup (see
// internal/app/runtime_config_linux.go) and read by the smooth-scroll path.
//
// It is a second slot rather than a share of platform/linux's because the two
// packages are on opposite sides of the adapter boundary: scroll injection
// lives here, beside the element walk, while cursor movement lives there, and
// neither imports the other.
//
// A nil provider (tests, or before wiring) means "no config", so callers fall
// back to the direct, unanimated scroll.
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
