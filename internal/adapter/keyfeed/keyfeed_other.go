//go:build !darwin && !linux && !windows

package keyfeed

import "github.com/y3owk1n/neru/internal/derrors"

// postKey reports that this platform has no key-injection path. No shipped
// target lands here; the key_feed entry in ports.PlatformCapabilities reports
// stub to match.
func postKey(_ string) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"key feeding is only supported on macOS, Linux and Windows",
	)
}
