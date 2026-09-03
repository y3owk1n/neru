//go:build linux

package linux

import (
	"errors"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// errScrollDeviceGone is what the recorder answers when a test asks it to fail
// a batch.
var errScrollDeviceGone = errors.New("uinput scroll device is gone")

// uinputScrollRecorder stands in for the uinput scroll device, remembering the
// batch each axis was handed.
type uinputScrollRecorder struct {
	batches []uinputScrollBatchRecord
	err     error
}

type uinputScrollBatchRecord struct {
	axis   int
	values []int
}

func (r *uinputScrollRecorder) batch(axis int, values []int) error {
	if r.err != nil {
		return r.err
	}

	r.batches = append(r.batches, uinputScrollBatchRecord{
		axis:   axis,
		values: append([]int(nil), values...),
	})

	return nil
}

func withUinputScrollRecorder(t *testing.T, recorder *uinputScrollRecorder) {
	t.Helper()

	original := uinputScrollBatch
	uinputScrollBatch = recorder.batch

	t.Cleanup(func() { uinputScrollBatch = original })
}

// TestHyprlandScrollSession_HoldsTheModifierAcrossEveryChunk is the reason this
// session exists rather than a press and release around each chunk: an
// application reads twenty presses of ctrl as twenty zoom gestures, not as one.
func TestHyprlandScrollSession_HoldsTheModifierAcrossEveryChunk(t *testing.T) {
	modifiers := &modifierRecorder{}
	withModifierRecorder(t, modifiers)

	scrolls := &uinputScrollRecorder{}
	withUinputScrollRecorder(t, scrolls)

	session := &hyprlandScrollSession{}

	pressed, err := pressWaylandModifiers(action.ModCtrl)
	if err != nil {
		t.Fatalf("pressWaylandModifiers() = %v, want no error", err)
	}

	session.pressed = pressed

	for range 3 {
		err = session.inject(0, -scrollPixelsPerNotch)
		if err != nil {
			t.Fatalf("inject() = %v, want no error", err)
		}
	}

	if len(modifiers.events) != 1 {
		t.Fatalf("key events during the animation = %v, want only the press", modifiers.events)
	}

	session.close()

	assertEvents(t, modifiers.events, []string{ctrlDown, ctrlUp})

	if len(scrolls.batches) != 3 {
		t.Fatalf("scroll batches = %v, want one per chunk", scrolls.batches)
	}
}

// TestHyprlandScrollSession_Inject pins the chunk-to-notch conversion, which is
// what granularity promises the animator: it only ever hands whole notches, and
// each one goes out as a single event of the right sign on the right axis.
func TestHyprlandScrollSession_Inject(t *testing.T) {
	tests := []struct {
		name   string
		deltaX float64
		deltaY float64
		want   []uinputScrollBatchRecord
	}{
		{
			name:   "a chunk worth nothing sends nothing",
			want:   nil,
			deltaX: 0,
			deltaY: 0,
		},
		{
			name:   "scrolling down sends one negative vertical notch",
			deltaY: -scrollPixelsPerNotch,
			want: []uinputScrollBatchRecord{
				{axis: uinputScrollAxisVertical, values: []int{-1}},
			},
		},
		{
			name:   "a three-notch chunk sends three events",
			deltaY: 3 * scrollPixelsPerNotch,
			want: []uinputScrollBatchRecord{
				{axis: uinputScrollAxisVertical, values: []int{1, 1, 1}},
			},
		},
		{
			name:   "both axes go out vertical first",
			deltaX: -scrollPixelsPerNotch,
			deltaY: scrollPixelsPerNotch,
			want: []uinputScrollBatchRecord{
				{axis: uinputScrollAxisVertical, values: []int{1}},
				{axis: uinputScrollAxisHorizontal, values: []int{-1}},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := &uinputScrollRecorder{}
			withUinputScrollRecorder(t, recorder)

			session := &hyprlandScrollSession{}

			err := session.inject(testCase.deltaX, testCase.deltaY)
			if err != nil {
				t.Fatalf("inject() = %v, want no error", err)
			}

			assertScrollBatches(t, recorder.batches, testCase.want)
		})
	}
}

// TestHyprlandScrollSession_InjectReportsAFailedBatch keeps a dead scroll
// device from reading as a scroll: the animator stops on an injection error,
// and a nil here would ease its way through the whole curve moving nothing.
func TestHyprlandScrollSession_InjectReportsAFailedBatch(t *testing.T) {
	withUinputScrollRecorder(t, &uinputScrollRecorder{err: errScrollDeviceGone})

	session := &hyprlandScrollSession{}

	err := session.inject(0, scrollPixelsPerNotch)
	if !errors.Is(err, errScrollDeviceGone) {
		t.Fatalf("inject() = %v, want the recorder's refusal", err)
	}
}

// TestHyprlandScrollSession_Granularity pins the unit the animator rounds its
// curve to. uinput scrolling is whole REL_WHEEL clicks, so a session claiming
// to be continuous would have every sub-notch chunk silently dropped.
func TestHyprlandScrollSession_Granularity(t *testing.T) {
	session := &hyprlandScrollSession{}

	if got := session.granularity(); got != scrollPixelsPerNotch {
		t.Fatalf("granularity() = %v, want %v", got, float64(scrollPixelsPerNotch))
	}
}

func assertScrollBatches(t *testing.T, got, want []uinputScrollBatchRecord) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("scroll batches = %v, want %v", got, want)
	}

	for batch := range want {
		if got[batch].axis != want[batch].axis ||
			!slices.Equal(got[batch].values, want[batch].values) {
			t.Fatalf("scroll batches = %v, want %v", got, want)
		}
	}
}
