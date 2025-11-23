package state

import (
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
)

func TestNewAppState(t *testing.T) {
	state := NewAppState()

	if state == nil {
		t.Fatal("NewAppState() returned nil")
	}

	if !state.IsEnabled() {
		t.Error("Expected new state to be enabled by default")
	}

	if state.CurrentMode() != domain.ModeIdle {
		t.Errorf("Expected initial mode to be ModeIdle, got %v", state.CurrentMode())
	}
}

func TestAppState_EnableDisable(t *testing.T) {
	state := NewAppState()

	// Test Enable
	state.Disable()
	if state.IsEnabled() {
		t.Error("Expected state to be disabled")
	}

	state.Enable()
	if !state.IsEnabled() {
		t.Error("Expected state to be enabled")
	}

	// Test SetEnabled
	state.SetEnabled(false)
	if state.IsEnabled() {
		t.Error("Expected state to be disabled after SetEnabled(false)")
	}

	state.SetEnabled(true)
	if !state.IsEnabled() {
		t.Error("Expected state to be enabled after SetEnabled(true)")
	}
}

func TestAppState_Mode(t *testing.T) {
	state := NewAppState()

	modes := []domain.Mode{
		domain.ModeIdle,
		domain.ModeHints,
		domain.ModeGrid,
	}

	for _, mode := range modes {
		state.SetMode(mode)
		if state.CurrentMode() != mode {
			t.Errorf("Expected mode %v, got %v", mode, state.CurrentMode())
		}
	}
}

func TestAppState_HotkeysRegistered(t *testing.T) {
	state := NewAppState()

	if state.HotkeysRegistered() {
		t.Error("Expected hotkeys to not be registered initially")
	}

	state.SetHotkeysRegistered(true)
	if !state.HotkeysRegistered() {
		t.Error("Expected hotkeys to be registered")
	}

	state.SetHotkeysRegistered(false)
	if state.HotkeysRegistered() {
		t.Error("Expected hotkeys to not be registered")
	}
}

func TestAppState_ScreenChangeProcessing(t *testing.T) {
	state := NewAppState()

	if state.ScreenChangeProcessing() {
		t.Error("Expected screen change processing to be false initially")
	}

	state.SetScreenChangeProcessing(true)
	if !state.ScreenChangeProcessing() {
		t.Error("Expected screen change processing to be true")
	}

	state.SetScreenChangeProcessing(false)
	if state.ScreenChangeProcessing() {
		t.Error("Expected screen change processing to be false")
	}
}

func TestAppState_GridOverlayNeedsRefresh(t *testing.T) {
	state := NewAppState()

	if state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay refresh to be false initially")
	}

	state.SetGridOverlayNeedsRefresh(true)
	if !state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay to need refresh")
	}

	state.SetGridOverlayNeedsRefresh(false)
	if state.GridOverlayNeedsRefresh() {
		t.Error("Expected grid overlay to not need refresh")
	}
}

func TestAppState_HintOverlayNeedsRefresh(t *testing.T) {
	state := NewAppState()

	if state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay refresh to be false initially")
	}

	state.SetHintOverlayNeedsRefresh(true)
	if !state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay to need refresh")
	}

	state.SetHintOverlayNeedsRefresh(false)
	if state.HintOverlayNeedsRefresh() {
		t.Error("Expected hint overlay to not need refresh")
	}
}

func TestAppState_HotkeyRefreshPending(t *testing.T) {
	state := NewAppState()

	if state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh pending to be false initially")
	}

	state.SetHotkeyRefreshPending(true)
	if !state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh to be pending")
	}

	state.SetHotkeyRefreshPending(false)
	if state.HotkeyRefreshPending() {
		t.Error("Expected hotkey refresh to not be pending")
	}
}

// TestAppState_Concurrency tests thread-safe access to state.
func TestAppState_Concurrency(_ *testing.T) {
	state := NewAppState()
	var wg sync.WaitGroup

	// Concurrent reads and writes
	for range 100 {
		wg.Add(2)

		go func() {
			defer wg.Done()
			state.SetEnabled(true)
			_ = state.IsEnabled()
		}()

		go func() {
			defer wg.Done()
			state.SetMode(domain.ModeHints)
			_ = state.CurrentMode()
		}()
	}

	wg.Wait()
}

// Benchmark tests.
func BenchmarkAppState_GetSet(b *testing.B) {
	state := NewAppState()

	for b.Loop() {
		state.SetEnabled(true)
		_ = state.IsEnabled()
		state.SetMode(domain.ModeHints)
		_ = state.CurrentMode()
	}
}

func BenchmarkAppState_ConcurrentAccess(b *testing.B) {
	state := NewAppState()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			state.SetEnabled(true)
			_ = state.IsEnabled()
		}
	})
}
