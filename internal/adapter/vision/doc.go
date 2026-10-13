// Package vision implements ports.VisionPort, OCR-based element detection for
// hints, backed by the macOS Vision framework.
//
// It is used only when hints.strategy is "vision"; the accessibility tree is
// the default source everywhere. adapter_darwin.go captures the frontmost
// window and runs text-recognition, rectangle-detection, and saliency requests,
// then classifier.go assigns roles heuristically.
//
// adapter_linux.go answers the same port with two native pieces: real pixels —
// wlr-screencopy on wlroots compositors, XGetImage on X11 — and tesseract
// through platform/linux/ocr.c, bound with #cgo pkg-config the way every other
// native dependency in the tree is. adapter_windows.go captures with BitBlt and
// reads text with Windows.Media.Ocr. Both find button shapes with the contour
// detector over the same frame (rectangles.go), and neither runs saliency.
// adapter_other.go has no capture and refuses everything.
//
// The contour strategy is a third detector, the wl-kbptr algorithm in the
// contour subpackage, which is pure Go and platform-neutral; darwin, linux and
// windows each feed it a frame from their own capture backend, so it lands
// wherever capture does.
//
// Captured pixels and recognized text are both screen content. Neither is
// logged, written to disk, or held past the call that asked for it; the native
// buffers behind them are wiped before they are released, and the engine is
// cleared of the frame before each recognition returns.
package vision
