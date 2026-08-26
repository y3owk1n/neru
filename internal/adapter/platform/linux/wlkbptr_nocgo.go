//go:build linux && !cgo

package linux

import "image"

// DetectWLKBPTRTargets is a stub for non-CGO Linux builds.
func DetectWLKBPTRTargets(_ *image.RGBA, _ float64) []image.Rectangle {
	return nil
}
