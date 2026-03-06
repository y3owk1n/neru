package state_test

import (
	"image"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func TestNewCursorState(t *testing.T) {
	state := state.NewCursorState()
	if state == nil {
		t.Fatal("NewCursorState() returned nil")
	}

	if state.IsCaptured() {
		t.Error("Expected new state to not be captured")
	}
}

func TestCursorState_Capture(t *testing.T) {
	state := state.NewCursorState()

	pos := image.Point{X: 100, Y: 200}
	bounds := image.Rect(0, 0, 1920, 1080)

	state.Capture(pos, bounds)

	if !state.IsCaptured() {
		t.Error("Expected state to be captured after Capture()")
	}

	gotPos := state.InitialPosition()
	if gotPos != pos {
		t.Errorf("Expected position %v, got %v", pos, gotPos)
	}

	gotBounds := state.InitialScreenBounds()
	if gotBounds != bounds {
		t.Errorf("Expected bounds %v, got %v", bounds, gotBounds)
	}
}

func TestCursorState_Reset(t *testing.T) {
	state := state.NewCursorState()

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

	if state.ShouldMoveCursor() {
		t.Error("Expected ShouldMoveCursor to be false after Reset()")
	}
}

func TestCursorState_ShouldMoveCursor(t *testing.T) {
	tests := []struct {
		name     string
		captured bool
		skipOnce bool
		expected bool
	}{
		{
			name:     "captured, not skipped",
			captured: true,
			skipOnce: false,
			expected: true,
		},
		{
			name:     "not captured",
			captured: false,
			skipOnce: false,
			expected: false,
		},
		{
			name:     "captured, skip once set",
			captured: true,
			skipOnce: true,
			expected: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			state := state.NewCursorState()

			if testCase.captured {
				state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
			}

			if testCase.skipOnce {
				state.SkipNextRestore()
			}

			got := state.ShouldMoveCursor()
			if got != testCase.expected {
				t.Errorf("ShouldMoveCursor() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestCursorState_SkipNextRestore(t *testing.T) {
	state := state.NewCursorState()
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	if !state.ShouldMoveCursor() {
		t.Error("Expected ShouldMoveCursor to be true before SkipNextRestore()")
	}

	state.SkipNextRestore()

	if state.ShouldMoveCursor() {
		t.Error("Expected ShouldMoveCursor to be false after SkipNextRestore()")
	}

	// Reset should clear the skip flag
	state.Reset()
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	if !state.ShouldMoveCursor() {
		t.Error("Expected ShouldMoveCursor to be true after Reset() and Capture()")
	}
}

// TestCursorState_Concurrency tests thread-safe access to cursor state.
func TestCursorState_Concurrency(_ *testing.T) {
	state := state.NewCursorState()

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

			_ = state.InitialPosition()
			_ = state.InitialScreenBounds()
		}()

		go func() {
			defer waitGroup.Done()

			_ = state.ShouldMoveCursor()
		}()
	}

	waitGroup.Wait()
}

// Stress tests for robustness.

// TestCursorState_RapidStateTransitions tests rapid capture/release cycles.
func TestCursorState_RapidStateTransitions(t *testing.T) {
	state := state.NewCursorState()

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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			state := state.NewCursorState()
			state.Capture(testCase.pos, testCase.bounds)

			if !state.IsCaptured() {
				t.Error("State should be captured")
			}

			gotPos := state.InitialPosition()
			if gotPos != testCase.pos {
				t.Errorf("InitialPosition() = %v, want %v", gotPos, testCase.pos)
			}

			gotBounds := state.InitialScreenBounds()
			if gotBounds != testCase.bounds {
				t.Errorf("InitialScreenBounds() = %v, want %v", gotBounds, testCase.bounds)
			}
		})
	}
}

// TestCursorState_ConcurrentStressTest performs intensive concurrent operations.
func TestCursorState_ConcurrentStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	state := state.NewCursorState()

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

				_ = state.InitialPosition()
				_ = state.InitialScreenBounds()
				_ = state.ShouldMoveCursor()

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
	state := state.NewCursorState()

	// Invariant 1: After Reset(), IsCaptured() should be false
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))
	state.Reset()

	if state.IsCaptured() {
		t.Error("Invariant violated: IsCaptured() should be false after Reset()")
	}

	// Invariant 2: ShouldMoveCursor() should be false when not captured
	if state.ShouldMoveCursor() {
		t.Error("Invariant violated: ShouldMoveCursor() should be false when not captured")
	}

	// Invariant 3: After Capture(), IsCaptured() should be true
	state.Capture(image.Point{X: 100, Y: 200}, image.Rect(0, 0, 1920, 1080))

	if !state.IsCaptured() {
		t.Error("Invariant violated: IsCaptured() should be true after Capture()")
	}

	// Invariant 4: ShouldMoveCursor() should be true when captured
	if !state.ShouldMoveCursor() {
		t.Error(
			"Invariant violated: ShouldMoveCursor() should be true when captured",
		)
	}

	// Invariant 5: SkipNextRestore should affect ShouldMoveCursor()
	state.SkipNextRestore()

	if state.ShouldMoveCursor() {
		t.Error("Invariant violated: ShouldMoveCursor() should be false after SkipNextRestore()")
	}
}
