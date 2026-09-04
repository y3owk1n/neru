//go:build windows

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/config/loader"
)

func configurePlatformRuntimeConfigProviders(cfgService *loader.Service, _ *zap.Logger) {
	windows.SetConfigProvider(cfgService)
}
