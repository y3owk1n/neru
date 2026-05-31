//go:build !darwin

// Package keyfeed posts keyboard input directly to the host operating system.
package keyfeed

import derrors "github.com/y3owk1n/neru/internal/core/errors"

// Feed is not implemented outside macOS yet.
func Feed(_ string) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"key feeding is only supported on macOS",
	)
}

// NormalizeKeyForFeed normalizes a key string for feeding to the OS.
// Not supported outside macOS.
func NormalizeKeyForFeed(key string) (string, error) {
	return "", derrors.New(
		derrors.CodeNotSupported,
		"key feeding is only supported on macOS",
	)
}
