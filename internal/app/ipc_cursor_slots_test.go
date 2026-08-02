//nolint:testpackage // Tests the private cursor-slot handlers.
package app

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// cursorSlotHarness drives the two cursor actions against a fake pointer, so a
// test can say where the cursor is and see where a restore put it.
type cursorSlotHarness struct {
	handler *IPCControllerActions
	// position is what the next save captures.
	position image.Point
	// moves records every point a restore moved the cursor to.
	moves []image.Point
}

func newCursorSlotHarness(t *testing.T) *cursorSlotHarness {
	t.Helper()

	harness := &cursorSlotHarness{}

	system := &portmocks.MockSystemPort{
		CursorPositionFunc: func(context.Context) (image.Point, error) {
			return harness.position, nil
		},
		MoveCursorToPointFunc: func(_ context.Context, point image.Point, _ bool) error {
			harness.moves = append(harness.moves, point)

			return nil
		},
	}

	harness.handler = NewIPCControllerActions(
		services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			system,
			zap.NewNop(),
		),
		nil,
		nil,
		state.NewAppState(),
		nil,
		state.NewCursorSlots(),
		zap.NewNop(),
	)

	return harness
}

// save captures the given position into the slot the args name.
func (h *cursorSlotHarness) save(t *testing.T, at image.Point, args ...string) ipc.Response {
	t.Helper()

	h.position = at

	return h.handler.handleAction(context.Background(), ipc.Command{
		Args: append([]string{"save_cursor_pos"}, args...),
	})
}

func (h *cursorSlotHarness) restore(t *testing.T, args ...string) ipc.Response {
	t.Helper()

	return h.handler.handleAction(context.Background(), ipc.Command{
		Args: append([]string{"restore_cursor_pos"}, args...),
	})
}

func TestCursorSlots_SaveAndRestoreRoundTrip(t *testing.T) {
	t.Parallel()

	harness := newCursorSlotHarness(t)

	saved := image.Point{X: 120, Y: 340}

	if resp := harness.save(t, saved); !resp.Success {
		t.Fatalf("save_cursor_pos = %+v, want success", resp)
	}

	if resp := harness.restore(t); !resp.Success {
		t.Fatalf("restore_cursor_pos = %+v, want success", resp)
	}

	if len(harness.moves) != 1 || harness.moves[0] != saved {
		t.Fatalf("cursor moves = %v, want exactly [%v]", harness.moves, saved)
	}
}

// The reason named slots exist. A sequence that saves the cursor and then
// invokes one that also saves used to lose the outer save silently, and the
// outer restore then moved the cursor somewhere it never asked for.
func TestCursorSlots_NamedSlotDoesNotClobberTheDefault(t *testing.T) {
	t.Parallel()

	harness := newCursorSlotHarness(t)

	outer := image.Point{X: 10, Y: 20}
	inner := image.Point{X: 900, Y: 800}

	harness.save(t, outer)
	harness.save(t, inner, "--slot=inner")

	harness.restore(t, "--slot=inner")
	harness.restore(t)

	want := []image.Point{inner, outer}
	if len(harness.moves) != len(want) {
		t.Fatalf("cursor moves = %v, want %v", harness.moves, want)
	}

	for idx, point := range want {
		if harness.moves[idx] != point {
			t.Fatalf("cursor moves = %v, want %v", harness.moves, want)
		}
	}
}

// Restoring consumes the slot. A second restore is not an error — it simply has
// nothing to move to, which keeps a sequence that restores twice from being a
// failure a caller has to special-case.
func TestCursorSlots_RestoreConsumesTheSlot(t *testing.T) {
	t.Parallel()

	harness := newCursorSlotHarness(t)

	harness.save(t, image.Point{X: 5, Y: 5})
	harness.restore(t)

	resp := harness.restore(t)
	if !resp.Success || resp.Code != ipc.CodeOK {
		t.Fatalf("second restore_cursor_pos = %+v, want success", resp)
	}

	if len(harness.moves) != 1 {
		t.Fatalf("cursor moves = %v, want the second restore to move nothing", harness.moves)
	}
}

// "default" is an ordinary slot name, not a reserved word: naming it and
// passing no flag have to be the same request, or a config author would have
// two ways to mean one slot and no way to tell which they used.
func TestCursorSlots_DefaultSlotIsAnOrdinaryName(t *testing.T) {
	t.Parallel()

	harness := newCursorSlotHarness(t)

	saved := image.Point{X: 42, Y: 43}

	harness.save(t, saved, "--slot="+state.DefaultCursorSlot)

	if resp := harness.restore(t); !resp.Success {
		t.Fatalf("restore_cursor_pos = %+v, want success", resp)
	}

	if len(harness.moves) != 1 || harness.moves[0] != saved {
		t.Fatalf("cursor moves = %v, want [%v]", harness.moves, saved)
	}
}

func TestCursorSlots_RejectsInvalidSlotNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		slot string
	}{
		{name: "empty", slot: "--slot="},
		{name: "leading digit", slot: "--slot=9lives"},
		{name: "looks like a flag", slot: "--slot=--center"},
		{name: "dot", slot: "--slot=a.b"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			harness := newCursorSlotHarness(t)

			resp := harness.save(t, image.Point{X: 1, Y: 1}, testCase.slot)
			if resp.Success || resp.Code != ipc.CodeInvalidInput {
				t.Fatalf("save_cursor_pos %s = %+v, want invalid input", testCase.slot, resp)
			}
		})
	}
}

// The flag belongs to these two actions and nothing else, so an action that
// does not take it must say so rather than ignore it.
func TestCursorSlots_SlotIsRejectedOnOtherActions(t *testing.T) {
	t.Parallel()

	harness := newCursorSlotHarness(t)

	resp := harness.handler.handleAction(context.Background(), ipc.Command{
		Args: []string{leftClick, "--slot=here"},
	})

	if resp.Success || resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("left_click --slot = %+v, want invalid input", resp)
	}
}
