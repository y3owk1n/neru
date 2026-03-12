//go:build windows

package app

import "go.uber.org/zap"

// initializePlatformLogger is a no-op on Windows.
func initializePlatformLogger(_ *zap.Logger) {}
