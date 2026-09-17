//go:build linux && cgo

package linux

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// TestPhysicalLift_Release_WaitsForTheDragThatEndedUnderAScroll pins the two
// orderings a scroll and a drag can overlap in. Whichever ends first must
// leave the chord up for the other, and the last one out puts it back.
func TestPhysicalLift_Release_WaitsForTheDragThatEndedUnderAScroll(t *testing.T) {
	tests := []struct {
		name string
		run  func(lift *physicalLift)
	}{
		{
			name: "drag ends before the scroll",
			run: func(lift *physicalLift) {
				lift.acquire()
				globalWlrootsPointerState.SetDown(action.ButtonLeft, image.Point{}, 0)
				globalWlrootsPointerState.Clear(action.ButtonLeft)
				lift.restoreUnlessHeld(action.ButtonLeft)

				if !lift.lifted {
					t.Fatal("drag release restored the chord under a running scroll")
				}

				lift.release()
			},
		},
		{
			name: "scroll ends before the drag",
			run: func(lift *physicalLift) {
				globalWlrootsPointerState.SetDown(action.ButtonLeft, image.Point{}, 0)
				lift.acquire()
				lift.release()

				if !lift.lifted {
					t.Fatal("scroll release restored the chord under a held drag")
				}

				globalWlrootsPointerState.Clear(action.ButtonLeft)
				lift.restoreUnlessHeld(action.ButtonLeft)
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Cleanup(func() { globalWlrootsPointerState.Clear(action.ButtonLeft) })

			// No proxy runs under the test, so the lift is marked by hand:
			// what is under test is who restores, not the proxy.
			lift := &physicalLift{lifted: true}

			testCase.run(lift)

			if lift.lifted {
				t.Fatal("the last action out left the chord lifted")
			}

			if lift.holds != 0 {
				t.Fatalf("holds = %d after every action ended, want 0", lift.holds)
			}
		})
	}
}
