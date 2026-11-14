//go:build linux

package app

import (
	"go.uber.org/zap"

	nativelinux "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/config/loader"
)

func configurePlatformRuntimeConfigProviders(cfgService *loader.Service, logger *zap.Logger) {
	// Scroll injection lives in the accessibility backend. It reads
	// smooth_scroll through platform/linux's slot, so this one call covers both.
	linux.SetAnimationSource(cfgService.Animation)
	// The scroll path logs which backend it fell back to from code below any
	// struct that carries a logger, so its logger comes from a slot.
	nativelinux.SetLogger(logger)
}
