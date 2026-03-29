//go:build !darwin

package app

import "github.com/y3owk1n/neru/internal/config"

func configurePlatformRuntimeConfigProviders(_ *config.Service) {}
