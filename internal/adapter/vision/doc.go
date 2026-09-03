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
// native dependency in the tree is. It answers the *text* half only: rectangle
// detection and saliency have no OCR equivalent, so hints.vision.detect_rectangles
// and the four rectangle_* options are declared macOS-only and Linux vision is
// text-only (docs/adr/0013). adapter_other.go has neither half and refuses
// everything.
//
// The contour strategy is a third detector, the wl-kbptr algorithm in the
// contour subpackage, which is pure C and platform-neutral; darwin and linux
// each feed it a frame from their own capture backend, so it lands wherever
// capture does.
//
// Captured pixels and recognized text are both screen content. Neither is
// logged, written to disk, or held past the call that asked for it; the native
// buffers behind them are wiped before they are released, and the engine is
// cleared of the frame before each recognition returns.
package vision
