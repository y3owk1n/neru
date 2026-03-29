//go:build darwin

package darwin

import (
	"sync"

	"github.com/y3owk1n/neru/internal/config"
)

// ConfigProvider exposes the current config snapshot for native runtime helpers.
type ConfigProvider interface {
	Get() *config.Config
}

var (
	configProviderMu sync.RWMutex
	configProvider   ConfigProvider
)

// SetConfigProvider updates the runtime config provider used by mouse helpers.
func SetConfigProvider(provider ConfigProvider) {
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
