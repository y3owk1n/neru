//go:build linux

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux/inject"
	"github.com/y3owk1n/neru/internal/config/loader"
)

func configurePlatformRuntimeConfigProviders(cfgService *loader.Service, logger *zap.Logger) {
	// Scroll injection lives in platform/linux/inject. It reads smooth_scroll
	// through platform/linux's slot, so this one call covers both.
	linux.SetAnimationSource(cfgService.Animation)
	// The scroll path logs which backend it fell back to from code below any
	// struct that carries a logger, so its logger comes from a slot.
	inject.SetLogger(logger)
}
