//go:build !darwin

// Package space provides functions for focusing Mission Control spaces.
//
// The macOS implementation uses the synthetic high-velocity horizontal
// dock swipe gesture technique (see space_darwin.go). This file is a stub
// used on non-macOS platforms where Mission Control does not exist.
package space

import derrors "github.com/y3owk1n/neru/internal/core/errors"

// FocusByIndex focuses the Mission Control space at the given 1-based
// index. Not supported outside macOS.
func FocusByIndex(_ int) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"space switching is only supported on macOS",
	)
}

// Count returns the total number of Mission Control spaces. Not
// supported outside macOS.
func Count() int {
	return 0
}
