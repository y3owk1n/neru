//go:build linux

package app

import (
	nativelinux "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/config/loader"
)

func configurePlatformRuntimeConfigProviders(cfgService *loader.Service) {
	linux.SetConfigProvider(cfgService)
	// Scroll injection lives in the accessibility backend rather than in
	// platform/linux, so smooth_scroll needs its own reader on that side of the
	// boundary. On darwin both paths are in one package and one call does.
	nativelinux.SetConfigProvider(cfgService)
}
