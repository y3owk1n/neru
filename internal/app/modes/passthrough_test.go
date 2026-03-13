//nolint:testpackage // Tests internal method modeModifierKeys which is private
package modes

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"go.uber.org/zap"
)

func TestModeModifierKeys_HintsIncludesExitAndActionBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.ModeExitKeys = []string{"Escape", "Ctrl+C"}
	cfg.Action.KeyBindings.LeftClick = "Cmd+L"
	cfg.Action.KeyBindings.MoveMouseUp = "Alt+K"
	cfg.Hints.BackspaceKey = "Ctrl+H"

	handler := &Handler{config: cfg}

	got := handler.modeModifierKeys(domain.ModeHints)
	want := []string{"Alt+K", "Cmd+L", "Ctrl+C", "Ctrl+H"}

	if !slices.Equal(got, want) {
		t.Fatalf("modeModifierKeys(ModeHints) = %v, want %v", got, want)
	}
}

func TestModeModifierKeys_ScrollIncludesOnlyModifierBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.ModeExitKeys = []string{"Escape", "Ctrl+C"}
	cfg.Scroll.KeyBindings = map[string][]string{
		"scroll_up":   {"k", "Up"},
		"go_top":      {"gg", "Cmd+Up"},
		"go_bottom":   {"Shift+G", "Cmd+Down"},
		"page_up":     {"Ctrl+U", "PageUp"},
		"page_down":   {"Ctrl+D", "PageDown"},
		"scroll_left": {"h"},
	}

	handler := &Handler{config: cfg}

	got := handler.modeModifierKeys(domain.ModeScroll)
	want := []string{"Cmd+Down", "Cmd+Up", "Ctrl+C", "Ctrl+D", "Ctrl+U"}

	if !slices.Equal(got, want) {
		t.Fatalf("modeModifierKeys(ModeScroll) = %v, want %v", got, want)
	}
}

func TestHandlePassthroughLocked_IgnoresStaleSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.PassthroughUnboundedKeys = true

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := &Handler{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 2,
	}

	handler.handlePassthroughLocked(domain.ModeHints, 1)

	if handler.refreshHintsTimer != nil {
		t.Fatal("expected stale passthrough callback to be ignored")
	}
}

func TestPassthroughCallbackFor_CapturesModeSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.PassthroughUnboundedKeys = true

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := &Handler{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 1,
	}

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

	handler := &Handler{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 3,
	}

	handler.handlePassthroughLocked(domain.ModeHints, 3)

	if handler.refreshHintsTimer == nil {
		t.Fatal("expected matching passthrough callback to schedule a hint refresh")
	}

	handler.refreshHintsTimer.Stop()
}
