//go:build darwin

//nolint:testpackage // tests unexported cgoSlot internals
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

				// Encourage interleaving so concurrent Set(0)/Set(1) updates can
				// invalidate this snapshot before stillValid checks it.
				runtime.Gosched()

				if !slot.stillValid(generation) {
					staleSeen.Add(1)

					continue
				}

				if value != 0 {
					calls.Add(1)
				}
			}
		}()
	}

	go func() {
		defer waitGroup.Done()

		for range 200 {
			slot.Set(1)
			runtime.Gosched()
			slot.Set(0)
			runtime.Gosched()
		}
	}()

	waitGroup.Wait()

	if calls.Load() == 0 {
		t.Fatal("expected at least one dispatch while slot holds a value")
	}

	if staleSeen.Load() == 0 {
		t.Fatal("expected at least one stale snapshot invalidated by concurrent clear")
	}
}
