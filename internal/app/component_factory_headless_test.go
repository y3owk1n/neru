//go:build darwin || linux

package app_test

// The component factory must not build render overlays when the overlay
// manager has no surface for them: every draw through them reaches a native
// API (CGo, X11) that the manager never opened. Which managers those are is
// something the manager states, not something the factory infers from a window
// handle it happens to be able to see.

import (
	"testing"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
)

// declaredHeadlessManager hands out a window handle and still declares itself
// headless. The two disagree deliberately: the factory has to believe the
// declaration, which is the whole difference between asking and guessing.
type declaredHeadlessManager struct {
	overlay.NoOpManager
}

// notAWindow stands in for a handle the factory must ignore. It is never
// dereferenced — the point is only that it is not nil.
var notAWindow = new(byte)

func (m *declaredHeadlessManager) WindowPtr() unsafe.Pointer {
	return unsafe.Pointer(notAWindow)
}

func (m *declaredHeadlessManager) Headless() bool { return true }

// lightTheme is a fixed appearance so building a hint style needs no system.
type lightTheme struct{}

func (lightTheme) IsDarkMode() bool { return false }

func TestComponentFactory_SkipsOverlaysADeclaredHeadlessManagerCannotDraw(t *testing.T) {
	factory := app.NewComponentFactory(
		config.DefaultConfig(),
		zap.NewNop(),
		&declaredHeadlessManager{},
		lightTheme{},
	)

	component, err := factory.CreateHintsComponent(app.ComponentCreationOptions{
		OverlayType: "hints",
	})
	if err != nil {
		t.Fatalf("CreateHintsComponent returned %v, want nil", err)
	}

	if component.Overlay != nil {
		t.Error(
			"a hints overlay was built for a manager that declared itself headless; " +
				"the factory read the window handle instead of the declaration",
		)
	}
}
