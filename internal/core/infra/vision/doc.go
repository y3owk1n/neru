// Package vision implements ports.VisionPort, OCR-based element detection for
// hints, backed by the macOS Vision framework.
//
// It is used only when hints.strategy is "vision"; the accessibility tree is
// the default source everywhere. adapter_darwin.go captures the frontmost
// window and runs text-recognition, rectangle-detection, and saliency requests,
// then classifier.go assigns roles heuristically. adapter_other.go returns
// CodeNotSupported: no other platform has an equivalent, and the vision entry
// in ports.PlatformCapabilities reports stub there.
package vision
