//go:build windows

package windows

import (
	"sync"

	"github.com/y3owk1n/neru/internal/config"
)

// configProvider is a process-global runtime config source for the Windows
// system adapter, mirroring the darwin and linux config-provider slots. It is
// set once at daemon startup (see internal/app/runtime_config_windows.go) and
// read by the smooth cursor path. A nil provider (tests, or before wiring)
// means "no config", so callers fall back to the direct, non-animated move.
var (
	configProviderMu sync.RWMutex
	configProvider   config.Provider
)

// SetConfigProvider updates the runtime config provider used by the Windows
// system adapter's smooth-cursor path.
func SetConfigProvider(provider config.Provider) {
	configProviderMu.Lock()
	configProvider = provider
	configProviderMu.Unlock()
}

// currentWindowsConfig returns the latest config snapshot, or nil when no
// provider has been wired yet.
func currentWindowsConfig() *config.Config {
	configProviderMu.RLock()

	provider := configProvider

	configProviderMu.RUnlock()

	if provider == nil {
		return nil
	}

	return provider.Get()
}
