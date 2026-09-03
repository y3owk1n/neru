//go:build darwin

package darwin_test

import (
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/darwin"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/config"
)

// TestManager_Destroy_DoesNotOverlapSetSharingType pins the one concurrency
// pair this manager has. Every other call reaches it from the mode handler,
// but the screen-share callback is published on a goroutine of its own
// (state.AppState notifies with `go callback(...)`), so it can be inside
// SetSharingType while a shutdown releases the windows underneath it. The
// adapter's destroyed flag cannot stop that one — a callback already past the
// check is in the backend — so shareMu has to.
//
// A zero-value Manager is used deliberately: Init is a process-global
// singleton that creates a real NSWindow, and the state that matters here is
// what both methods touch, not what is drawn on it. That is two fields, not
// one — the window handle, and the render components SetSharingType propagates
// to, which Destroy forgets through Base's unguarded setters — so a grid
// overlay is registered before the race starts to put the second within reach.
// A component with no window of its own is safe to drive: every native call
// either makes returns immediately on a null handle. Run under -race, an
// unsynchronized Destroy is a write to each field against SetSharingType's
// read of it.
func TestManager_Destroy_DoesNotOverlapSetSharingType(t *testing.T) {
	const callers = 8

	mgr := &darwin.Manager{}
	mgr.UseGridOverlay(grid.NewOverlayWithWindow(config.DefaultConfig().Grid, nil, nil))

	var waitGroup sync.WaitGroup

	start := make(chan struct{})

	waitGroup.Add(callers)

	for caller := range callers {
		go func() {
			defer waitGroup.Done()

			<-start

			mgr.SetSharingType(caller%2 == 0)
		}()
	}

	waitGroup.Go(func() {
		<-start

		mgr.Destroy()
	})

	close(start)
	waitGroup.Wait()

	// Destroy is final and idempotent: a second one must not hand the same
	// handle to the native release twice, and a call arriving after it must
	// find nothing to touch.
	mgr.Destroy()
	mgr.SetSharingType(true)

	// The assertion the -race pass makes is that neither goroutine saw the
	// other's write; what is left to state here is the postcondition that
	// makes those writes safe to skip — a released manager holds no handle,
	// so everything reaching it afterwards has nothing to release or draw on.
	if !mgr.Headless() {
		t.Error("Headless() = false after Destroy(); the window handle outlived the release")
	}
}
