//go:build !darwin && !windows && !linux

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config/loader"
)

func configurePlatformRuntimeConfigProviders(_ *loader.Service, _ *zap.Logger) {}
