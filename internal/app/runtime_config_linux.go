//go:build linux

package app

import (
	"go.uber.org/zap"

	nativelinux "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/config/loader"
)

func configurePlatformRuntimeConfigProviders(cfgService *loader.Service, logger *zap.Logger) {
	linux.SetConfigProvider(cfgService)
	// Scroll injection lives in the accessibility backend rather than in
	// platform/linux, so smooth_scroll needs its own reader on that side of the
	// boundary. On darwin both paths are in one package and one call does.
	nativelinux.SetConfigProvider(cfgService)
	// Same slot shape for the logger: the scroll path reports which backend
	// it fell back to from below any struct that carries one.
	nativelinux.SetLogger(logger)
}
