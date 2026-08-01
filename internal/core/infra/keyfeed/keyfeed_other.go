//go:build !darwin && !linux

package keyfeed

import derrors "github.com/y3owk1n/neru/internal/core/errors"

// postKey reports that this platform has no key-injection path.
//
// Windows lands here today; the target is SendInput. The key_feed entry in
// ports.PlatformCapabilities reports stub to match.
func postKey(_ string) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"key feeding is only supported on macOS and Linux",
	)
}
