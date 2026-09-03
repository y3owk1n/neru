//go:build darwin

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/config/loader"
)

func configurePlatformRuntimeConfigProviders(cfgService *loader.Service, _ *zap.Logger) {
	darwin.SetConfigProvider(cfgService)
}
