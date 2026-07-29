//go:build linux

package app

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

func configurePlatformRuntimeConfigProviders(cfgService *config.Service) {
	linux.SetConfigProvider(cfgService)
}
