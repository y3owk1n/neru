// Package textinput implements ports.TextInputPort, the native text field used
// by hint search.
//
// Adapter is the platform-agnostic wrapper; the TextInput it delegates to is
// selected by build tag. Only macOS has a native field (textinput_darwin.go, an
// NSTextField overlay). textinput_other.go reports started == false so callers
// fall back to reading the event tap's key stream — a documented best-effort
// degrade rather than a CodeNotSupported error. Availability is reported
// through the text_input entry in ports.PlatformCapabilities.
package textinput
