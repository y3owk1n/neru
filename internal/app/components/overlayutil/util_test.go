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
	if started := manager.StartResizeOperation(func(uint64, uint64) {}); !started {
		t.Fatal("expected resize operation to start")
	}

	// Hold registry lock to force Cleanup to wait at the registry phase.
	callbackManagerRegistryMu.Lock()
	// Hold callbackMu so Cleanup goroutine lines up on this lock first.
	manager.callbackMu.Lock()

	done := make(chan struct{})
	go func() {
		manager.Cleanup()
		close(done)
	}()

	// Let Cleanup proceed.
	manager.callbackMu.Unlock()
	// Give the cleanup goroutine a chance to acquire/release callbackMu.
	time.Sleep(10 * time.Millisecond)

	// If Cleanup still holds callbackMu while waiting for registry lock, this
	// loop will time out. We expect callbackMu to be free at this point.
	acquired := false

	deadline := time.Now().Add(200 * time.Millisecond)

	for time.Now().Before(deadline) {
		if manager.callbackMu.TryLock() {
			manager.callbackMu.Unlock()
			acquired = true

			break
		}

		runtime.Gosched()
		time.Sleep(1 * time.Millisecond)
	}

	callbackManagerRegistryMu.Unlock()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("cleanup did not complete after releasing registry lock")
	}

	if !acquired {
		t.Fatal("cleanup appears to hold callbackMu while waiting for registry lock")
	}
}
