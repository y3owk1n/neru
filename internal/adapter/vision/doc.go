// Package vision implements ports.VisionPort, OCR-based element detection for
// hints.
//
// It is used only when hints.strategy is "vision". The accessibility tree is
// the default source everywhere. adapter_darwin.go captures with
// ScreenCaptureKit and adapter_linux.go with wlr-screencopy on wlroots
// compositors or XGetImage on X11. Both read text with yomi (yomi_read.go) and
// find button shapes with the contour detector over the same frame
// (detect_elements.go, rectangles.go), then the classifier in heuristics.go
// assigns roles. adapter_windows.go captures with BitBlt and reads the same
// way. adapter_other.go has no capture and refuses everything.
//
// The contour and segment strategies are two more detectors: the wl-kbptr
// algorithm in the contour subpackage, and the segment subpackage, which
// splits a frame into controls, pictures and text labels. Both are pure Go and
// platform-neutral. Darwin, linux and windows each feed them a frame from
// their own capture backend (targets.go), so they work wherever capture does.
//
// Captured pixels and recognized text are both screen content. Neither is
// logged, written to disk, or held past the call that asked for it. The native
// buffers behind them are wiped before they are released, and the engine is
// cleared of the frame before each recognition returns.
package vision
