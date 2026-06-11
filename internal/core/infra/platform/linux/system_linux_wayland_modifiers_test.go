//go:build linux

//nolint:testpackage // These tests exercise unexported dispatcher helpers directly.
package linux

import (
	"errors"
	"testing"
)

const ctrlModifier = "ctrl"

var errReleaseFailed = errors.New("release failed")

func TestWlrootsModifierDispatcherDeduplicatesNestedUsage(t *testing.T) {
	t.Parallel()

	type event struct {
		modifier string
		isDown   bool
	}

	var events []event

	dispatcher := newWlrootsModifierDispatcher(func(modifier string, isDown bool) error {
		events = append(events, event{modifier: modifier, isDown: isDown})

		return nil
	})

	err := dispatcher.event(ctrlModifier, true)
	if err != nil {
		t.Fatalf("first down error = %v", err)
	}

	err = dispatcher.event(ctrlModifier, true)
	if err != nil {
		t.Fatalf("nested down error = %v", err)
	}

	err = dispatcher.event(ctrlModifier, false)
	if err != nil {
		t.Fatalf("nested up error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("event count after nested release = %d, want 1", len(events))
	}

	err = dispatcher.event(ctrlModifier, false)
	if err != nil {
		t.Fatalf("final up error = %v", err)
	}

	want := []event{
		{modifier: ctrlModifier, isDown: true},
		{modifier: ctrlModifier, isDown: false},
	}

	if len(events) != len(want) {
		t.Fatalf("final event count = %d, want %d", len(events), len(want))
	}

	for index, got := range events {
		if got != want[index] {
			t.Fatalf("event[%d] = %+v, want %+v", index, got, want[index])
		}
	}
}

func TestWlrootsModifierDispatcherRetriesFailedFinalRelease(t *testing.T) {
	t.Parallel()

	failRelease := true
	releaseAttempts := 0

	dispatcher := newWlrootsModifierDispatcher(func(modifier string, isDown bool) error {
		if modifier != ctrlModifier {
			t.Fatalf("modifier = %q, want ctrl", modifier)
		}

		if !isDown {
			releaseAttempts++

			if failRelease {
				failRelease = false

				return errReleaseFailed
			}
		}

		return nil
	})

	err := dispatcher.event(ctrlModifier, true)
	if err != nil {
		t.Fatalf("down error = %v", err)
	}

	err = dispatcher.event(ctrlModifier, false)
	if err == nil {
		t.Fatal("first up error = nil, want failure")
	}

	err = dispatcher.event(ctrlModifier, false)
	if err != nil {
		t.Fatalf("retry up error = %v", err)
	}

	if releaseAttempts != 2 {
		t.Fatalf("release attempts = %d, want 2", releaseAttempts)
	}
}

func TestWlrootsModifierDispatcherAllowsCleanupReleaseWithoutTrackedDown(t *testing.T) {
	t.Parallel()

	calls := 0

	dispatcher := newWlrootsModifierDispatcher(func(modifier string, isDown bool) error {
		calls++

		if modifier != ctrlModifier {
			t.Fatalf("modifier = %q, want ctrl", modifier)
		}

		if isDown {
			t.Fatal("unexpected down event during cleanup release test")
		}

		return nil
	})

	err := dispatcher.event(ctrlModifier, false)
	if err != nil {
		t.Fatalf("cleanup release error = %v", err)
	}

	if calls != 1 {
		t.Fatalf("cleanup release calls = %d, want 1", calls)
	}
}
