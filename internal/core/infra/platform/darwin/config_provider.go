//go:build darwin

package darwin

import (
	"sync"

	"github.com/y3owk1n/neru/internal/config"
)

var (
	configProviderMu sync.RWMutex
	configProvider   config.Provider
)

// SetConfigProvider updates the runtime config provider used by mouse helpers.
func SetConfigProvider(provider config.Provider) {
	configProviderMu.Lock()
	defer configProviderMu.Unlock()

	configProvider = provider
}

func currentConfig() *config.Config {
	configProviderMu.RLock()
	defer configProviderMu.RUnlock()

	if configProvider == nil {
		return nil
	}

	return configProvider.Get()
}
