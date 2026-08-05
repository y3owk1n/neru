//go:build darwin || linux

package app

import "github.com/y3owk1n/neru/internal/adapter/overlay"

// headless returns true when the overlay manager declares that it has no
// surface to render on, in which case callers must not build render overlays
// against it. On macOS that is a crash guard — the overlays are CGo objects
// built on a window that was never created. Elsewhere they are inert stubs, so
// the guard instead keeps the app from holding components the manager has no
// way to draw.
//
// The manager states it through the optional HeadlessReporter capability,
// reached by type assertion; a manager that does not implement it can render.
func (f *ComponentFactory) headless() bool {
	reporter, ok := f.overlayManager.(overlay.HeadlessReporter)

	return ok && reporter.Headless()
}
