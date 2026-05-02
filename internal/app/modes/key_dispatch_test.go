//nolint:testpackage // These tests validate unexported handler behavior directly.
package modes

import (
	"context"
	"image"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
)

const testSafariBundleID = "com.apple.Safari"

type recordingMode struct {
	keys chan string
}

func (m *recordingMode) Activate(ModeActivationOptions) {}
func (m *recordingMode) HandleKey(key string)           { m.keys <- key }
func (m *recordingMode) Exit()                          {}
func (m *recordingMode) ModeType() domain.Mode          { return domain.ModeRecursiveGrid }

func TestHandleKeyPressUsesStickyStrippedKeyForBindings(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	mode := &recordingMode{keys: make(chan string, 1)}
	hotkeyActions := make(chan string, 1)

	handler := &Handler{
		config: &configpkg.Config{
			RecursiveGrid: configpkg.RecursiveGridConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"Ctrl+C": {"exit"},
				},
			},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		modes: map[domain.Mode]Mode{
			domain.ModeRecursiveGrid: mode,
		},
		screenBounds: image.Rect(0, 0, 100, 100),
		executeHotkeyAction: func(_, actionStr string) error {
			hotkeyActions <- actionStr

			return nil
		},
	}
	handler.modifierState.Toggle(action.ModCtrl)

	handler.HandleKeyPress("Ctrl+c")

	select {
	case got := <-mode.keys:
		if got != "c" {
			t.Fatalf("mode key = %q, want %q", got, "c")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stripped mode key")
	}

	select {
	case got := <-hotkeyActions:
		t.Fatalf("sticky modifier leaked into hotkey action %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHandleKeyPressUsesRawHintActionHotkeyWhenStickyShiftStripped(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	mode := &recordingMode{keys: make(chan string, 1)}
	hotkeyActions := make(chan string, 1)

	handler := &Handler{
		config: &configpkg.Config{
			Hints: configpkg.HintsConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"Shift+L": {"action left_click"},
				},
			},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{
				FocusedAppBundleIDFunc: func(context.Context) (string, error) {
					return testSafariBundleID, nil
				},
			},
			&portmocks.MockOverlayPort{},
			&portmocks.SystemMock{},
			zap.NewNop(),
		),
		modes: map[domain.Mode]Mode{
			domain.ModeHints: mode,
		},
		screenBounds: image.Rect(0, 0, 100, 100),
		executeHotkeyAction: func(_, actionStr string) error {
			hotkeyActions <- actionStr

			return nil
		},
	}
	handler.modifierState.Toggle(action.ModShift)

	handler.HandleKeyPress("Shift+L")

	select {
	case got := <-hotkeyActions:
		if got != "action left_click" {
			t.Fatalf("hotkey action = %q, want %q", got, "action left_click")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for raw hint action hotkey")
	}

	select {
	case got := <-mode.keys:
		t.Fatalf("mode-specific key handler received %q, want hotkey to consume input", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHandleKeyPressUsesRawHintActionHotkeyWithActionAfterModeAction(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	mode := &recordingMode{keys: make(chan string, 1)}
	hotkeyActions := make(chan string, 2)

	handler := &Handler{
		config: &configpkg.Config{
			Hints: configpkg.HintsConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"Shift+L": {"idle", "action left_click"},
				},
			},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{
				FocusedAppBundleIDFunc: func(context.Context) (string, error) {
					return "com.apple.Safari", nil
				},
			},
			&portmocks.MockOverlayPort{},
			&portmocks.SystemMock{},
			zap.NewNop(),
		),
		modes: map[domain.Mode]Mode{
			domain.ModeHints: mode,
		},
		screenBounds: image.Rect(0, 0, 100, 100),
		executeHotkeyAction: func(_, actionStr string) error {
			hotkeyActions <- actionStr

			return nil
		},
	}
	handler.modifierState.Toggle(action.ModShift)

	handler.HandleKeyPress("Shift+L")

	for _, want := range []string{"idle", "action left_click"} {
		select {
		case got := <-hotkeyActions:
			if got != want {
				t.Fatalf("hotkey action = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for hotkey action %q", want)
		}
	}

	select {
	case got := <-mode.keys:
		t.Fatalf("mode-specific key handler received %q, want hotkey to consume input", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHandleKeyPressHonorsDisabledAppHintActionOverride(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	mode := &recordingMode{keys: make(chan string, 1)}
	hotkeyActions := make(chan string, 1)

	handler := &Handler{
		config: &configpkg.Config{
			Hints: configpkg.HintsConfig{
				Hotkeys: map[string]configpkg.StringOrStringArray{
					"Shift+L": {"action left_click"},
				},
				AppConfigs: []configpkg.AppConfig{
					{
						BundleID: testSafariBundleID,
						Hotkeys: map[string]configpkg.StringOrStringArray{
							"Shift+L": {configpkg.DisabledSentinel},
						},
					},
				},
			},
		},
		logger:        zap.NewNop(),
		appState:      appState,
		modifierState: state.NewModifierState(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{
				FocusedAppBundleIDFunc: func(context.Context) (string, error) {
					return testSafariBundleID, nil
				},
			},
			&portmocks.MockOverlayPort{},
			&portmocks.SystemMock{},
			zap.NewNop(),
		),
		modes: map[domain.Mode]Mode{
			domain.ModeHints: mode,
		},
		screenBounds: image.Rect(0, 0, 100, 100),
		executeHotkeyAction: func(_, actionStr string) error {
			hotkeyActions <- actionStr

			return nil
		},
	}
	handler.modifierState.Toggle(action.ModShift)

	handler.HandleKeyPress("Shift+L")

	select {
	case got := <-mode.keys:
		if got != "L" {
			t.Fatalf("mode key = %q, want %q", got, "L")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stripped mode key")
	}

	select {
	case got := <-hotkeyActions:
		t.Fatalf("disabled app override still dispatched hotkey action %q", got)
	case <-time.After(50 * time.Millisecond):
	}
}
