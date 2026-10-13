// Package textinput implements ports.TextInputPort, the native text field used
// by hint search.
//
// Adapter is the platform-agnostic wrapper; the TextInput it delegates to is
// selected by build tag. macOS has a text view in a key panel
// (textinput_darwin.go), and Linux a layer-shell surface on Wayland
// compositors that offer one (textinput_linux_cgo.go). Everywhere else,
// and wherever that surface cannot open, StartHintSearchSession reports
// started == false so callers fall back to reading the event tap's key stream —
// a documented best-effort degrade rather than a CodeNotSupported error. Availability is reported
// through the text_input entry in ports.PlatformCapabilities.
package textinput
