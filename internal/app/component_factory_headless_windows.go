//go:build windows

package app

// headless always returns false on Windows, and the Windows overlay manager
// deliberately does not declare the HeadlessReporter capability at all. Its
// surface is recreated on demand at draw time, so having no window when
// components are built is not a verdict on whether it can render — and the
// render overlays here are inert stubs that only store the handle they are
// given, so building them against a missing surface costs nothing.
func (f *ComponentFactory) headless() bool {
	return false
}
