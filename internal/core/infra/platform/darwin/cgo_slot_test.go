//go:build darwin

//nolint:testpackage // tests unexported cgoSlot internals
package darwin

import (
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
		slot    cgoSlot[int]
		okCalls atomic.Int32
	)

	done := make(chan struct{})
	go func() {
		defer close(done)

		for i := 1; i <= 100; i++ {
			slot.withValid(func(v int) {
				if v == i {
					okCalls.Add(1)
				}
			})
		}
	}()

	for i := 1; i <= 100; i++ {
		slot.Set(i)
	}

	<-done

	if okCalls.Load() == 0 {
		t.Fatal("expected at least one valid dispatch under concurrent set")
	}
}
