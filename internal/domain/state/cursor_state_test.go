package state

import (
	"image"
	"sync"
	"testing"
)

func TestNewCursorState(t *testing.T) {
	tests := []struct {
		name           string
		restoreEnabled bool
	}{
		{
			name:           "restore enabled",
			restoreEnabled: true,
		},
		{
			name:           "restore disabled",
			restoreEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewCursorState(tt.restoreEnabled)

			if state == nil {
				t.Fatal("NewCursorState() returned nil")
			}

			if state.IsRestoreEnabled() != tt.restoreEnabled {
				t.Errorf(
					"Expected restore enabled = %v, got %v",
					tt.restoreEnabled,
					state.IsRestoreEnabled(),
				)
			}

			if state.IsCaptured() {
				t.Error("Expected new state to not be captured")
			}
		})
	}
}

func TestCursorState_Capture(t *testing.T) {
	state := NewCursorState(true)

	pos := image.Point{X: 100, Y: 200}
	bounds := image.Rect(0, 0, 1920, 1080)

	state.Capture(pos, bounds)

	if !state.IsCaptured() {
		t.Error("Expected state to be captured after Capture()")
	}

	gotPos := state.GetInitialPosition()
	if gotPos != pos {
		t.Errorf("Expected position %v, got %v", pos, gotPos)
	}

	gotBounds := state.GetInitialScreenBounds()
	if gotBounds != bounds {
		t.Errorf("Expected bounds %v, got %v", bounds, gotBounds)
	}
}

func TestCursorState_Reset(t *testing.T) {
	state := NewCursorState(true)

	// Capture some state
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
	state.SkipNextRestore()

	if !state.IsCaptured() {
		t.Error("Expected state to be captured before reset")
	}

	// Reset
	state.Reset()

	if state.IsCaptured() {
		t.Error("Expected state to not be captured after Reset()")
	}

	if state.ShouldRestore() {
		t.Error("Expected ShouldRestore to be false after Reset()")
	}
}

func TestCursorState_ShouldRestore(t *testing.T) {
	tests := []struct {
		name           string
		restoreEnabled bool
		captured       bool
		skipOnce       bool
		expected       bool
	}{
		{
			name:           "restore enabled, captured, not skipped",
			restoreEnabled: true,
			captured:       true,
			skipOnce:       false,
			expected:       true,
		},
		{
			name:           "restore disabled",
			restoreEnabled: false,
			captured:       true,
			skipOnce:       false,
			expected:       false,
		},
		{
			name:           "not captured",
			restoreEnabled: true,
			captured:       false,
			skipOnce:       false,
			expected:       false,
		},
		{
			name:           "skip once set",
			restoreEnabled: true,
			captured:       true,
			skipOnce:       true,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewCursorState(tt.restoreEnabled)

			if tt.captured {
				state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
			}

			if tt.skipOnce {
				state.SkipNextRestore()
			}

			got := state.ShouldRestore()
			if got != tt.expected {
				t.Errorf("ShouldRestore() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCursorState_SetRestoreEnabled(t *testing.T) {
	state := NewCursorState(false)

	if state.IsRestoreEnabled() {
		t.Error("Expected restore to be disabled initially")
	}

	state.SetRestoreEnabled(true)

	if !state.IsRestoreEnabled() {
		t.Error("Expected restore to be enabled after SetRestoreEnabled(true)")
	}

	state.SetRestoreEnabled(false)

	if state.IsRestoreEnabled() {
		t.Error("Expected restore to be disabled after SetRestoreEnabled(false)")
	}
}

func TestCursorState_SkipNextRestore(t *testing.T) {
	state := NewCursorState(true)
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	if !state.ShouldRestore() {
		t.Error("Expected ShouldRestore to be true before SkipNextRestore()")
	}

	state.SkipNextRestore()

	if state.ShouldRestore() {
		t.Error("Expected ShouldRestore to be false after SkipNextRestore()")
	}

	// Reset should clear the skip flag
	state.Reset()
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	if !state.ShouldRestore() {
		t.Error("Expected ShouldRestore to be true after Reset() and Capture()")
	}
}

// TestCursorState_Concurrency tests thread-safe access to cursor state.
func TestCursorState_Concurrency(_ *testing.T) {
	state := NewCursorState(true)
	var wg sync.WaitGroup

	// Concurrent reads and writes
	for range 100 {
		wg.Add(3)

		go func() {
			defer wg.Done()
			state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
			_ = state.IsCaptured()
		}()

		go func() {
			defer wg.Done()
			_ = state.GetInitialPosition()
			_ = state.GetInitialScreenBounds()
		}()

		go func() {
			defer wg.Done()
			state.SetRestoreEnabled(true)
			_ = state.ShouldRestore()
		}()
	}

	wg.Wait()
}

// Benchmark tests.
func BenchmarkCursorState_Capture(b *testing.B) {
	state := NewCursorState(true)
	pos := image.Point{X: 100, Y: 200}
	bounds := image.Rect(0, 0, 1920, 1080)

	for b.Loop() {
		state.Capture(pos, bounds)
	}
}

func BenchmarkCursorState_ShouldRestore(b *testing.B) {
	state := NewCursorState(true)
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	for b.Loop() {
		_ = state.ShouldRestore()
	}
}

func BenchmarkCursorState_ConcurrentAccess(b *testing.B) {
	state := NewCursorState(true)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
			_ = state.ShouldRestore()
		}
	})
}
