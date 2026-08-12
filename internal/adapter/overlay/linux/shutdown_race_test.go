//go:build linux

package linux

import (
	"image"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

const (
	// concurrentDestroyRounds is how many fresh Managers each case races. A
	// shutdown race only reports when the two goroutines actually overlap, so
	// one round proves nothing; the case repeats until they do.
	concurrentDestroyRounds = 200

	raceScreenWidth  = 800
	raceScreenHeight = 600
	raceGridSide     = 2

	racePointerFontSize = 20
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
		{
			name: "ShowSubgrid",
			call: func(mgr *Manager) {
				mgr.ShowSubgrid(firstGridCell(), grid.Style{})
			},
		},
	}
}

// firstGridCell is one cell of a grid the size of the fake screen this package's
// tests draw on, which is what opening a subgrid takes. It lives here rather
// than beside the subgrid tests because this file carries the broader build tag
// of the two, so both the cgo and the no-cgo build see it.
func firstGridCell() *domainGrid.Cell {
	return domainGrid.NewGrid(
		"ab",
		image.Rect(0, 0, raceScreenWidth, raceScreenHeight),
		zap.NewNop(),
	).AllCells()[0]
}

// gridPointerCalls are the two entry points grid mode's pointer stand-in
// travels on (#1463). They belong in a table of their own because they cancel
// no animation: each reads the backend pointer and dispatches through it, both
// inside renderMu, and that containment is the whole of what stops Destroy
// nilling the pointer between the two — a promoted method reached through a nil
// backend panics on the promotion (ADR 0010).
func gridPointerCalls() []cancelingCall {
	return []cancelingCall{
		{
			name: "DrawGridPointer",
			call: func(mgr *Manager) {
				mgr.DrawGridPointer(
					manager.ModeGrid,
					image.Pt(raceScreenWidth/2, raceScreenHeight/2),
					manager.PointerAppearance{
						FillColor:  "#123456",
						FontFamily: "Test Sans",
						Char:       "✛",
						FontSize:   racePointerFontSize,
					},
				)
			},
		},
		{
			name: "HideGridPointer",
			call: func(mgr *Manager) { mgr.HideGridPointer(manager.ModeGrid) },
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

// The grid pointer takes the same shape as the calls above and none of their
// cancel: what it has to survive is Destroy nilling the backend pointer between
// the nil-test and the dispatch, which only holding renderMu across both
// prevents. Without this the property is asserted by reading the code alone,
// and the case above exercises the cancel-outside-the-lock calls instead — a
// different hazard, on a different pair of lines.
func TestManager_Destroy_RacedByGridPointerCalls(t *testing.T) {
	t.Parallel()

	for _, backend := range attachedBackends() {
		for _, entry := range gridPointerCalls() {
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
