package modes

import (
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

func TestModeModifierKeys_HintsIncludesModifierHotkeys(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys = map[string]config.StringOrStringArray{
		"Cmd+L": {"action left_click"},
		"Alt+K": {"action move_mouse_relative --dx=0 --dy=-10"},
		"k":     {"action scroll_up"},
	}

	got := modeModifierKeys(cfg.ResolveKeymap(config.ModeNameHints, ""))
	want := []string{
		config.CanonicalHotkeyForPlatform("Alt+K"),
		config.CanonicalHotkeyForPlatform("Cmd+L"),
	}

	if !slices.Equal(got, want) {
		t.Fatalf("modeModifierKeys(ModeHints) = %v, want %v", got, want)
	}
}

func TestModeModifierKeys_ScrollIncludesOnlyModifierHotkeys(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Scroll.Hotkeys = map[string]config.StringOrStringArray{
		"k":        {"action scroll_up"},
		"Cmd+Up":   {"action go_top"},
		"Cmd+Down": {"action go_bottom"},
		"gg":       {"action go_top"},
	}

	got := modeModifierKeys(cfg.ResolveKeymap(config.ModeNameScroll, ""))
	want := []string{
		config.CanonicalHotkeyForPlatform("Cmd+Down"),
		config.CanonicalHotkeyForPlatform("Cmd+Up"),
	}

	if !slices.Equal(got, want) {
		t.Fatalf("modeModifierKeys(ModeScroll) = %v, want %v", got, want)
	}
}

func TestHandlePassthroughLocked_IgnoresStaleSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.PassthroughUnboundedKeys = true

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := newHandlerWithState(handlerState{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 2,
	})

	handler.passthroughTick(domain.ModeHints, 1)

	if handler.refreshHintsTimer != nil {
		t.Fatal("expected stale passthrough callback to be ignored")
	}
}

func TestPassthroughCallbackFor_CapturesModeSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.PassthroughUnboundedKeys = true

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := newHandlerWithState(handlerState{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 1,
	})

	callback := handler.passthroughCallbackFor(domain.ModeHints, true)
	if callback == nil {
		t.Fatal("expected passthrough callback")
	}

	handler.modeSession = 2

	appState.SetMode(domain.ModeGrid)

	callback()

	if handler.refreshHintsTimer != nil {
		t.Fatal("expected captured passthrough callback to ignore a later mode session")
	}
}

func TestHandlePassthroughLocked_SchedulesHintRefreshForMatchingSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.PassthroughUnboundedKeys = true

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := newHandlerWithState(handlerState{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 3,
	})

	handler.passthroughTick(domain.ModeHints, 3)

	if handler.refreshHintsTimer == nil {
		t.Fatal("expected matching passthrough callback to schedule a hint refresh")
	}

	handler.refreshHintsTimer.Stop()
}
