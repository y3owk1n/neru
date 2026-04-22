//go:build linux

package linux

import (
	"errors"
	"testing"
)

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

	if err := dispatcher.event("ctrl", true); err != nil {
		t.Fatalf("first down error = %v", err)
	}

	if err := dispatcher.event("ctrl", true); err != nil {
		t.Fatalf("nested down error = %v", err)
	}

	if err := dispatcher.event("ctrl", false); err != nil {
		t.Fatalf("nested up error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("event count after nested release = %d, want 1", len(events))
	}

	if err := dispatcher.event("ctrl", false); err != nil {
		t.Fatalf("final up error = %v", err)
	}

	want := []event{
		{modifier: "ctrl", isDown: true},
		{modifier: "ctrl", isDown: false},
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
		if modifier != "ctrl" {
			t.Fatalf("modifier = %q, want ctrl", modifier)
		}

		if !isDown {
			releaseAttempts++
			if failRelease {
				failRelease = false

				return errors.New("release failed")
			}
		}

		return nil
	})

	if err := dispatcher.event("ctrl", true); err != nil {
		t.Fatalf("down error = %v", err)
	}

	if err := dispatcher.event("ctrl", false); err == nil {
		t.Fatal("first up error = nil, want failure")
	}

	if err := dispatcher.event("ctrl", false); err != nil {
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

		if modifier != "ctrl" {
			t.Fatalf("modifier = %q, want ctrl", modifier)
		}

		if isDown {
			t.Fatal("unexpected down event during cleanup release test")
		}

		return nil
	})

	if err := dispatcher.event("ctrl", false); err != nil {
		t.Fatalf("cleanup release error = %v", err)
	}

	if calls != 1 {
		t.Fatalf("cleanup release calls = %d, want 1", calls)
	}
}
