//go:build darwin

package app

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

func configurePlatformRuntimeConfigProviders(cfgService *config.Service) {
	darwin.SetConfigProvider(cfgService)
}
