package state_test

import (
	"image"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/state"
)

func TestCursorSlots_SaveAndTake(t *testing.T) {
	t.Parallel()

	slots := state.NewCursorSlots()

	if _, ok := slots.Take("missing"); ok {
		t.Fatal("Take() on an empty store reported a position")
	}

	want := image.Point{X: 7, Y: 9}
	slots.Save("home", want)

	got, ok := slots.Take("home")
	if !ok {
		t.Fatal("Take() did not find the saved position")
	}

	if got != want {
		t.Fatalf("Take() = %v, want %v", got, want)
	}

	// Taking consumes it.
	if _, ok := slots.Take("home"); ok {
		t.Fatal("Take() found the position a previous Take should have consumed")
	}
}

func TestCursorSlots_SlotsAreIndependent(t *testing.T) {
	t.Parallel()

	slots := state.NewCursorSlots()

	first := image.Point{X: 1, Y: 1}
	second := image.Point{X: 2, Y: 2}

	slots.Save("first", first)
	slots.Save("second", second)
	slots.Save("first", first) // A repeat save must not disturb the other slot.

	if got, _ := slots.Take("second"); got != second {
		t.Fatalf("Take(second) = %v, want %v", got, second)
	}

	if got, _ := slots.Take("first"); got != first {
		t.Fatalf("Take(first) = %v, want %v", got, first)
	}
}

// Exactly one caller may win a slot. Reading and clearing separately would let
// two concurrent restores both see it occupied and both move the cursor.
func TestCursorSlots_TakeIsExclusive(t *testing.T) {
	t.Parallel()

	const racers = 16

	slots := state.NewCursorSlots()
	slots.Save("contested", image.Point{X: 3, Y: 4})

	var (
		group     sync.WaitGroup
		winnersMu sync.Mutex
		winners   int
	)

	start := make(chan struct{})

	for range racers {
		group.Go(func() {
			<-start

			if _, ok := slots.Take("contested"); ok {
				winnersMu.Lock()
				winners++
				winnersMu.Unlock()
			}
		})
	}

	close(start)
	group.Wait()

	if winners != 1 {
		t.Fatalf("Take() succeeded for %d racers, want exactly 1", winners)
	}
}

func TestCursorSlots_Snapshot(t *testing.T) {
	t.Parallel()

	slots := state.NewCursorSlots()

	empty := slots.Snapshot()
	if empty == nil {
		t.Fatal("Snapshot() = nil, want an empty map so callers always have something to encode")
	}

	if len(empty) != 0 {
		t.Fatalf("Snapshot() = %v, want empty", empty)
	}

	slots.Save("a", image.Point{X: 1, Y: 2})

	snapshot := slots.Snapshot()
	if len(snapshot) != 1 || snapshot["a"] != (image.Point{X: 1, Y: 2}) {
		t.Fatalf("Snapshot() = %v, want one entry for a", snapshot)
	}

	// The snapshot is a copy: mutating it must not reach the store.
	delete(snapshot, "a")

	if _, ok := slots.Take("a"); !ok {
		t.Fatal("mutating a snapshot removed the slot from the store")
	}
}
