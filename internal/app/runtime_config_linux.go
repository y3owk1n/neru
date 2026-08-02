//go:build linux

package app

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/config"
)

func configurePlatformRuntimeConfigProviders(cfgService *config.Service) {
	linux.SetConfigProvider(cfgService)
}
