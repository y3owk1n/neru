//go:build darwin

package app

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/config"
)

func configurePlatformRuntimeConfigProviders(cfgService *config.Service) {
	darwin.SetConfigProvider(cfgService)
}
