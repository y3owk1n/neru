package accessibility

import "github.com/y3owk1n/neru/internal/config"

// ConfigProvider exposes the current config snapshot for runtime infrastructure.
type ConfigProvider interface {
	Get() *config.Config
}

func currentConfig(provider ConfigProvider) *config.Config {
	if provider == nil {
		return nil
	}

	return provider.Get()
}
