//go:build darwin

package darwin

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCgoSlotInterfaceNilDoesNotPanic(t *testing.T) {
	type handler interface {
		Handle()
	}

	var slot cgoSlot[handler]

	slot.Set(nil)

	if _, _, ok := slot.snapshot(); ok {
		t.Fatal("expected empty snapshot for nil interface")
	}
}

func TestCgoSlotFuncZeroDoesNotPanic(t *testing.T) {
	var slot cgoSlot[func()]

	slot.Set(func() {})

	if _, _, ok := slot.snapshot(); !ok {
		t.Fatal("expected active snapshot for non-nil func")
	}

	slot.Set(nil)

	if _, _, ok := slot.snapshot(); ok {
		t.Fatal("expected empty snapshot after nil func clear")
	}
}

func TestCgoSlotSetInvalidatesPriorSnapshot(t *testing.T) {
	var slot cgoSlot[int]

	slot.Set(1)

	_, gen, ok := slot.snapshot()
	if !ok {
		t.Fatal("expected snapshot")
	}

	slot.Set(0)

	if slot.stillValid(gen) {
		t.Fatal("expected generation to be invalid after clear")
	}
}

func TestCgoSlotWithValidAsyncRejectsStaleGeneration(t *testing.T) {
	var (
		slot  cgoSlot[int]
		calls atomic.Int32
	)

	slot.Set(1)
	slot.Set(0)

	slot.withValidAsync(func(_ int) {
		calls.Add(1)
	})

	time.Sleep(20 * time.Millisecond)

	if got := calls.Load(); got != 0 {
		t.Fatalf("expected withValidAsync to drop dispatch after Set(0), got %d calls", got)
	}
}

// TestCgoSlotDispatchesAValidSnapshot pins the half of the generation guard
// that says yes: a snapshot nothing invalidated must actually reach its
// callback, on both dispatch paths.
//
// Every other dispatch assertion in this file is negative — "want 0 calls" —
// so without this one a stillValid that rejected everything would pass the
// whole package. The async wait is bounded rather than slept on: the timeout
// is a failure bound, not a race window, so a loaded runner is slow here, not
// red.
func TestCgoSlotDispatchesAValidSnapshot(t *testing.T) {
	var (
		slot      cgoSlot[int]
		syncCalls atomic.Int32
	)

	slot.Set(1)

	slot.withValid(func(value int) {
		if value != 1 {
			t.Errorf("withValid dispatched value %d, want 1", value)
		}

		syncCalls.Add(1)
	})

	if got := syncCalls.Load(); got != 1 {
		t.Fatalf("withValid dispatched %d times for a valid snapshot, want 1", got)
	}

	dispatched := make(chan int, 1)

	slot.withValidAsync(func(value int) {
		dispatched <- value
	})

	select {
	case value := <-dispatched:
		if value != 1 {
			t.Fatalf("withValidAsync dispatched value %d, want 1", value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("withValidAsync never dispatched a valid snapshot")
	}
}

// TestCgoSlotStaleSnapshotDoesNotDispatchAcrossGoroutines pins the guarantee
// the generation counter exists for: a reader holding a snapshot taken before
// a concurrent clear must not dispatch it.
//
// The interleaving is driven rather than hoped for. The reader takes its
// snapshot and hands the generation to the writer, the writer clears the slot
// and says so, and only then does the reader check validity — the exact order
// that makes the snapshot stale, on every schedule and every machine.
func TestCgoSlotStaleSnapshotDoesNotDispatchAcrossGoroutines(t *testing.T) {
	var (
		slot       cgoSlot[int]
		dispatched atomic.Int32
		waitGroup  sync.WaitGroup
	)

	slot.Set(1)

	held := make(chan uint64)
	cleared := make(chan struct{})

	waitGroup.Add(2)

	// The reader: what withValidAsync's goroutine does, with the window
	// between taking the snapshot and checking it opened deliberately.
	go func() {
		defer waitGroup.Done()

		value, generation, ok := slot.snapshot()
		if !ok {
			t.Error("snapshot of a slot holding a value reported empty")
			close(held)

			return
		}

		if value != 1 {
			t.Errorf("snapshot value = %d, want 1", value)
		}

		held <- generation

		<-cleared

		// The dispatch guard itself: the callback runs only for a generation
		// the slot still recognizes. By now the clear has happened, so this
		// must not fire.
		if slot.stillValid(generation) {
			dispatched.Add(1)
		}
	}()

	// The writer: clears only once the reader is holding its snapshot.
	go func() {
		defer waitGroup.Done()

		<-held
		slot.Set(0)
		close(cleared)
	}()

	waitGroup.Wait()

	if got := dispatched.Load(); got != 0 {
		t.Fatalf("stale snapshot dispatched %d times, want 0", got)
	}
}

// TestCgoSlotConcurrentSetAndDispatch soaks the snapshot/validity pair against
// a writer toggling the slot, under the race detector.
//
// It asserts only what holds under every schedule: an active snapshot always
// carries a value. Whether any given reader is caught mid-round by the writer
// is the scheduler's decision, so the counts are reported rather than asserted
// — asserting on them is what made this test fail green changes on a loaded
// runner. Both directions of the guard have deterministic tests above.
func TestCgoSlotConcurrentSetAndDispatch(t *testing.T) {
	var (
		slot      cgoSlot[int]
		calls     atomic.Int32
		staleSeen atomic.Int32
		waitGroup sync.WaitGroup
	)

	slot.Set(1)

	const dispatchers = 50
	waitGroup.Add(dispatchers + 1)

	for range dispatchers {
		go func() {
			defer waitGroup.Done()

			for range 200 {
				value, generation, ok := slot.snapshot()
				if !ok {
					continue
				}

				if value == 0 {
					t.Error("an active snapshot carried a cleared value")

					return
				}

				// Encourage interleaving so concurrent Set(0)/Set(1) updates can
				// invalidate this snapshot before stillValid checks it.
				runtime.Gosched()

				if !slot.stillValid(generation) {
					staleSeen.Add(1)

					continue
				}

				calls.Add(1)
			}
		}()
	}

	go func() {
		defer waitGroup.Done()

		// Yield so readers can observe the initial slot.Set(1) before
		// we start toggling between 1 and 0.
		for range 50 {
			runtime.Gosched()
		}

		for range 200 {
			slot.Set(1)
			runtime.Gosched()
			slot.Set(0)
			runtime.Gosched()
		}
	}()

	waitGroup.Wait()

	t.Logf("%d snapshots dispatched, %d invalidated before their validity check",
		calls.Load(), staleSeen.Load())
}
