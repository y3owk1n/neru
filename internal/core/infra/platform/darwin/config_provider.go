//go:build darwin

package darwin

import "github.com/y3owk1n/neru/internal/config"

var configProviderSlot cgoSlot[config.Provider]

// SetConfigProvider updates the runtime config provider used by mouse helpers.
func SetConfigProvider(provider config.Provider) {
	configProviderSlot.Set(provider)
}

func currentConfig() *config.Config {
	var cfg *config.Config

	configProviderSlot.withValid(func(provider config.Provider) {
		cfg = provider.Get()
	})

	return cfg
}
