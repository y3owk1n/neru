package stickyindicator_test

import (
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// TestService_UpdateIndicatorPositionForwardsSymbols pins the draw call this
// service adds to the shared half.
func TestService_UpdateIndicatorPositionForwardsSymbols(t *testing.T) {
	t.Parallel()

	var gotX, gotY int

	var gotSymbols string

	overlay := &portmocks.MockOverlayPort{
		DrawStickyModifiersIndicatorFunc: func(x, y int, symbols string) {
			gotX, gotY, gotSymbols = x, y, symbols
		},
	}
	service := stickyindicator.NewService(nil, overlay)

	service.UpdateIndicatorPosition(4, 5, "⇧")

	if gotX != 4 || gotY != 5 || gotSymbols != "⇧" {
		t.Errorf(
			"DrawStickyModifiersIndicator got (%d,%d,%q), want (4,5,\"⇧\")",
			gotX, gotY, gotSymbols,
		)
	}
}

// TestService_ShowHideDriveTheStickyIndicator pins which indicator this
// service owns. Hiding it is one call: the backend owns whatever erasing the
// symbols costs, which is what used to be a draw-with-no-symbols in mode
// logic.
func TestService_ShowHideDriveTheStickyIndicator(t *testing.T) {
	t.Parallel()

	overlay := &portmocks.MockOverlayPort{}
	service := stickyindicator.NewService(nil, overlay)

	service.Show()

	if visible, asked := overlay.IndicatorVisible(ports.StickyModifiersIndicator); !asked ||
		!visible {
		t.Errorf("after Show(): visible=%v asked=%v, want both true", visible, asked)
	}

	service.Hide()

	if visible, asked := overlay.IndicatorVisible(ports.StickyModifiersIndicator); !asked ||
		visible {
		t.Errorf("after Hide(): visible=%v asked=%v, want asked and not visible", visible, asked)
	}

	if _, asked := overlay.IndicatorVisible(ports.ModeIndicator); asked {
		t.Error("the sticky indicator service touched the mode indicator")
	}
}

// TestService_WithoutOverlayDrawsNothing pins that a service built for an
// indicator that was never constructed is silent rather than a panic.
func TestService_WithoutOverlayDrawsNothing(t *testing.T) {
	t.Parallel()

	service := stickyindicator.NewService(nil, nil)

	service.Show()
	service.UpdateIndicatorPosition(1, 2, "⌘")
	service.Hide()

	if service.Overlay() != nil {
		t.Error("Overlay() = non-nil, want nil for a service with no overlay")
	}
}

func TestModifierSymbolsString(t *testing.T) {
	// The display symbol for ModCmd is platform-dependent:
	// "⌘" on macOS, "❖" on Linux.
	cmdSym := "❖"
	if runtime.GOOS == "darwin" {
		cmdSym = "⌘"
	}

	tests := []struct {
		name string
		mods action.Modifiers
		want string
	}{
		{"none", 0, ""},
		{"cmd", action.ModCmd, cmdSym},
		{"shift", action.ModShift, "⇧"},
		{"alt", action.ModAlt, "⌥"},
		{"ctrl", action.ModCtrl, "⌃"},
		{"cmd+shift", action.ModCmd | action.ModShift, cmdSym + "⇧"},
		{"all", action.ModCmd | action.ModShift | action.ModAlt | action.ModCtrl, cmdSym + "⇧⌥⌃"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := stickyindicator.ModifierSymbolsString(testCase.mods)
			if got != testCase.want {
				t.Errorf(
					"ModifierSymbolsString(%v) = %q, want %q",
					testCase.mods,
					got,
					testCase.want,
				)
			}
		})
	}
}
