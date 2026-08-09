//go:build linux

package linux

import (
	"image"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
)

const (
	// concurrentDestroyRounds is how many fresh Managers each case races. A
	// shutdown race only reports when the two goroutines actually overlap, so
	// one round proves nothing; the case repeats until they do.
	concurrentDestroyRounds = 200

	raceScreenWidth  = 800
	raceScreenHeight = 600
	raceGridSide     = 2
)

// attachedBackend builds a Manager with one backend attached. The overlay is a
// zero value on purpose: every backend method short-circuits on a nil native
// handle, so these cases need no display server and the only thing left under
// test is the Manager's own locking around the backend pointer.
type attachedBackend struct {
	name   string
	attach func() *Manager
}

func attachedBackends() []attachedBackend {
	return []attachedBackend{
		{
			name: "x11",
			attach: func() *Manager {
				return &Manager{backend: linuxOverlayBackendX11, x11: &x11Overlay{}}
			},
		},
		{
			name: "wlroots",
			attach: func() *Manager {
				return &Manager{
					backend: linuxOverlayBackendWaylandWlroots,
					wlroots: &wlrootsOverlay{},
				}
			},
		},
	}
}

// cancelingCall is one exported Manager entry point that stops a running
// animation before it takes renderMu, named so a failure says which one lost
// the race.
type cancelingCall struct {
	name string
	call func(*Manager)
}

func cancelingCalls() []cancelingCall {
	return []cancelingCall{
		{name: "Hide", call: func(mgr *Manager) { mgr.Hide() }},
		{name: "Clear", call: func(mgr *Manager) { mgr.Clear() }},
		{
			name: "DrawHintsWithStyle",
			call: func(mgr *Manager) {
				_ = mgr.DrawHintsWithStyle(nil, hints.StyleMode{})
			},
		},
		{
			name: "DrawRecursiveGrid",
			call: func(mgr *Manager) {
				dims := domain.GridDimensions{Rows: raceGridSide, Cols: raceGridSide}
				_ = mgr.DrawRecursiveGrid(
					image.Rect(0, 0, raceScreenWidth, raceScreenHeight),
					0,
					"ab", dims,
					"cd", dims,
					recursivegrid.Style{},
					recursivegrid.VirtualPointerState{},
				)
			},
		},
		{
			name: "DrawMonitorSelect",
			call: func(mgr *Manager) {
				_ = mgr.DrawMonitorSelect(nil, manager.MonitorSelectStyle{})
			},
		},
	}
}

// Destroy nils both backend pointers under renderMu, so a call that reads them
// beside the lock races it — and a call that reads twice, once to nil-test and
// once to dispatch, hands a nil backend to cancelAnimation and panics. The
// reasoning is on Manager.cancelBackendAnimation, which is what these calls now
// go through.
//
// Each round also asserts Destroy's post-condition: whatever raced it, the
// Manager must end up with no backend attached. The primary oracle, though, is
// the race detector plus the absence of a panic, so run this with -race —
// without the detector it only fails on the interleaving that actually
// dereferences nil, which is rare enough to look green.
func TestManager_Destroy_RacedByCancelingCalls(t *testing.T) {
	t.Parallel()

	for _, backend := range attachedBackends() {
		for _, entry := range cancelingCalls() {
			t.Run(backend.name+"/"+entry.name, func(t *testing.T) {
				t.Parallel()

				for range concurrentDestroyRounds {
					mgr := backend.attach()

					raceAgainstDestroy(mgr, entry.call)

					if !mgr.Headless() {
						t.Fatalf(
							"%s left a backend attached after Destroy returned; "+
								"the overlay would go on drawing into a torn-down surface",
							entry.name,
						)
					}
				}
			})
		}
	}
}

// raceAgainstDestroy releases one Manager call and Destroy together on two
// goroutines and returns once both have finished.
func raceAgainstDestroy(mgr *Manager, call func(*Manager)) {
	var released sync.WaitGroup

	released.Add(1)

	var finished sync.WaitGroup

	finished.Go(func() {
		released.Wait()
		call(mgr)
	})

	finished.Go(func() {
		released.Wait()
		mgr.Destroy()
	})

	released.Done()
	finished.Wait()
}
