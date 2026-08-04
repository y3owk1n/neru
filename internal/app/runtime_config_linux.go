//go:build linux

package app

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/config/loader"
)

func configurePlatformRuntimeConfigProviders(cfgService *loader.Service) {
	linux.SetConfigProvider(cfgService)
}
