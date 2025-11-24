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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := NewCursorState(test.restoreEnabled)

			if state == nil {
				t.Fatal("NewCursorState() returned nil")
			}

			if state.IsRestoreEnabled() != test.restoreEnabled {
				t.Errorf(
					"Expected restore enabled = %v, got %v",
					test.restoreEnabled,
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := NewCursorState(test.restoreEnabled)

			if test.captured {
				state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
			}

			if test.skipOnce {
				state.SkipNextRestore()
			}

			got := state.ShouldRestore()
			if got != test.expected {
				t.Errorf("ShouldRestore() = %v, want %v", got, test.expected)
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

	var waitGroup sync.WaitGroup

	// Concurrent reads and writes
	for range 100 {
		waitGroup.Add(3)

		go func() {
			defer waitGroup.Done()

			state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
			_ = state.IsCaptured()
		}()

		go func() {
			defer waitGroup.Done()

			_ = state.GetInitialPosition()
			_ = state.GetInitialScreenBounds()
		}()

		go func() {
			defer waitGroup.Done()

			state.SetRestoreEnabled(true)
			_ = state.ShouldRestore()
		}()
	}

	waitGroup.Wait()
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

// Stress tests for robustness.

// TestCursorState_RapidStateTransitions tests rapid capture/release cycles.
func TestCursorState_RapidStateTransitions(t *testing.T) {
	state := NewCursorState(true)

	// Perform 1000 rapid transitions
	for range 1000 {
		pos := image.Point{X: 100, Y: 200}
		bounds := image.Rect(0, 0, 1920, 1080)

		state.Capture(pos, bounds)

		if !state.IsCaptured() {
			t.Error("State should be captured after Capture()")
		}

		state.Reset()

		if state.IsCaptured() {
			t.Error("State should not be captured after Reset()")
		}
	}
}

// TestCursorState_ExtremeValues tests handling of extreme coordinate values.
func TestCursorState_ExtremeValues(t *testing.T) {
	tests := []struct {
		name   string
		pos    image.Point
		bounds image.Rectangle
	}{
		{
			name:   "maximum positive values",
			pos:    image.Point{X: 999999, Y: 999999},
			bounds: image.Rect(0, 0, 999999, 999999),
		},
		{
			name:   "negative values",
			pos:    image.Point{X: -1000, Y: -1000},
			bounds: image.Rect(-1000, -1000, 1000, 1000),
		},
		{
			name:   "zero values",
			pos:    image.Point{X: 0, Y: 0},
			bounds: image.Rect(0, 0, 0, 0),
		},
		{
			name:   "mixed extreme values",
			pos:    image.Point{X: -999999, Y: 999999},
			bounds: image.Rect(-999999, -999999, 999999, 999999),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := NewCursorState(true)
			state.Capture(test.pos, test.bounds)

			if !state.IsCaptured() {
				t.Error("State should be captured")
			}

			gotPos := state.GetInitialPosition()
			if gotPos != test.pos {
				t.Errorf("GetInitialPosition() = %v, want %v", gotPos, test.pos)
			}

			gotBounds := state.GetInitialScreenBounds()
			if gotBounds != test.bounds {
				t.Errorf("GetInitialScreenBounds() = %v, want %v", gotBounds, test.bounds)
			}
		})
	}
}

// TestCursorState_ConcurrentStressTest performs intensive concurrent operations.
func TestCursorState_ConcurrentStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	state := NewCursorState(true)

	var waitGroup sync.WaitGroup

	errors := make(chan error, 1000)

	// Run 1000 concurrent goroutines
	for index := range 1000 {
		waitGroup.Add(1)

		go func(index int) {
			defer waitGroup.Done()

			// Each goroutine performs multiple operations
			for j := range 10 {
				pos := image.Point{X: index * 10, Y: j * 10}
				bounds := image.Rect(0, 0, index*100, j*100)

				state.Capture(pos, bounds)

				// We cannot assert state.IsCaptured() here because another goroutine
				// might have called Reset() in the meantime.
				// The goal of this test is to ensure thread safety (no panics/races).

				_ = state.GetInitialPosition()
				_ = state.GetInitialScreenBounds()
				_ = state.ShouldRestore()

				state.Reset()
			}
		}(index)
	}

	waitGroup.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}

// TestCursorState_StateInvariants validates state invariants.
func TestCursorState_StateInvariants(t *testing.T) {
	state := NewCursorState(true)

	// Invariant 1: After Reset(), IsCaptured() should be false
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
	state.Reset()

	if state.IsCaptured() {
		t.Error("Invariant violated: IsCaptured() should be false after Reset()")
	}

	// Invariant 2: ShouldRestore() should match IsRestoreEnabled() when not captured
	state.SetRestoreEnabled(true)

	if state.ShouldRestore() {
		t.Error("Invariant violated: ShouldRestore() should be false when not captured")
	}

	// Invariant 3: After Capture(), IsCaptured() should be true
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	if !state.IsCaptured() {
		t.Error("Invariant violated: IsCaptured() should be true after Capture()")
	}

	// Invariant 4: ShouldRestore() should be true when captured and restore enabled
	if !state.ShouldRestore() {
		t.Error(
			"Invariant violated: ShouldRestore() should be true when captured and restore enabled",
		)
	}

	// Invariant 5: Disabling restore should affect ShouldRestore()
	state.SetRestoreEnabled(false)

	if state.ShouldRestore() {
		t.Error("Invariant violated: ShouldRestore() should be false when restore disabled")
	}
}
