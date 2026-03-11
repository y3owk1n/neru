//nolint:testpackage // Tests internal method modeModifierKeys which is private
package modes

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
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
