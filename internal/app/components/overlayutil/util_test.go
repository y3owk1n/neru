//nolint:testpackage // This test validates internal lock-order behavior.
package overlayutil

import (
	"runtime"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestCallbackManagerCleanup_ReleasesCallbackMutexBeforeRegistryLock(t *testing.T) {
	manager := NewCallbackManager(zap.NewNop())

	var allocatedID uint64
	if started := manager.StartResizeOperation(func(id uint64, _ uint64) {
		allocatedID = id
	}); !started {
		t.Fatal("expected resize operation to start")
	}

	// Eagerly release the callback ID after the test so the 30-second
	// deferred release timer doesn't leak global state into other tests.
	// releaseCallbackID is idempotent — the later timer firing is a no-op.
	t.Cleanup(func() { releaseCallbackID(allocatedID) })

	// Hold registry lock to force Cleanup to wait at the registry phase.
	callbackManagerRegistryMu.Lock()
	// Hold callbackMu so Cleanup goroutine lines up on this lock first.
	manager.callbackMu.Lock()

	done := make(chan struct{})
	go func() {
		manager.Cleanup()
		close(done)
	}()

	// Let Cleanup proceed past callbackMu.
	manager.callbackMu.Unlock()
	// Because we still hold callbackManagerRegistryMu, Cleanup cannot
	// finish — it will block on the registry lock after releasing
	// callbackMu. Poll TryLock until callbackMu is free, proving
	// Cleanup released it before acquiring the registry lock.
	// Use a generous deadline as a safety net only; under correct
	// behavior TryLock succeeds almost immediately.
	acquired := false

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if manager.callbackMu.TryLock() {
			manager.callbackMu.Unlock()

			acquired = true

			break
		}

		runtime.Gosched()
	}

	if !acquired {
		t.Fatal("cleanup appears to hold callbackMu while waiting for registry lock")
	}
	callbackManagerRegistryMu.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup did not complete after releasing registry lock")
	}
}
