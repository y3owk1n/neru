package accessibility

import "github.com/y3owk1n/neru/internal/config"

func currentConfig(provider config.Provider) *config.Config {
	if provider == nil {
		return nil
	}

	return provider.Get()
}
