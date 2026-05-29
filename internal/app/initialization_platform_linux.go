//go:build linux

package app

import "go.uber.org/zap"

// initializePlatformLogger is a no-op on Linux.
func initializePlatformLogger(_ *zap.Logger) {}

// platformQuit is a no-op on Linux.
func platformQuit() {}
