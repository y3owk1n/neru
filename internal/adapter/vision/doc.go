// Package vision implements ports.VisionPort, OCR-based element detection for
// hints, backed by the macOS Vision framework.
//
// It is used only when hints.strategy is "vision"; the accessibility tree is
// the default source everywhere. adapter_darwin.go captures the frontmost
// window and runs text-recognition, rectangle-detection, and saliency requests,
// then classifier.go assigns roles heuristically.
//
// The port has two halves, and off macOS they are not implemented together.
// adapter_linux.go captures real pixels — wlr-screencopy on wlroots
// compositors, XGetImage on X11 — but has no engine to recognize them, so
// DetectElements and Health report CodeNotSupported and the vision entry in
// ports.PlatformCapabilities stays stub: a partly-implemented strategy is not
// one a user can select. adapter_other.go has neither half and refuses
// everything.
//
// Captured pixels are never logged, never written to disk, and never held past
// the call that asked for them; the native buffers behind them are wiped before
// they are released.
package vision
