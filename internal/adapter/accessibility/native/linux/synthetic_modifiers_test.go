//go:build linux

package linux

import (
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// recordedModifier is one announcement the sink was given.
type recordedModifier struct {
	modifier action.Modifiers
	isDown   bool
}

// fakeSink stands in for the event tap: the injection path only ever tells it
// what is about to go out.
type fakeSink struct {
	mu       sync.Mutex
	recorded []recordedModifier
}

func (f *fakeSink) RememberSyntheticModifier(modifier action.Modifiers, isDown bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.recorded = append(f.recorded, recordedModifier{modifier: modifier, isDown: isDown})
}

func (f *fakeSink) events() []recordedModifier {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]recordedModifier(nil), f.recorded...)
}

// wireSink points the package at sink for the length of one test, and puts back
// whatever was there — the slot is process-global, like the config provider
// beside it.
func wireSink(t *testing.T, sink tap.SyntheticModifierSink) {
	t.Helper()

	syntheticModifierMu.Lock()
	previous := syntheticModifierSink
	syntheticModifierMu.Unlock()

	SetSyntheticModifierSink(sink)

	t.Cleanup(func() { SetSyntheticModifierSink(previous) })
}

// TestRecordSyntheticModifier_AnnouncesEachInjectionToTheWiredSink is the
// injection side of #1484: an X11 modifier key event Neru injects re-enters its
// own event tap indistinguishable from the user's, and a press followed by a
// release latches a sticky modifier nobody pressed. Announcing both halves is
// what lets the tap disown them.
func TestRecordSyntheticModifier_AnnouncesEachInjectionToTheWiredSink(t *testing.T) {
	sink := &fakeSink{}
	wireSink(t, sink)

	recordSyntheticModifier(action.ModCtrl, true)
	recordSyntheticModifier(action.ModCtrl, false)

	want := []recordedModifier{
		{modifier: action.ModCtrl, isDown: true},
		{modifier: action.ModCtrl, isDown: false},
	}

	got := sink.events()
	if len(got) != len(want) {
		t.Fatalf("sink saw %v, want the press and the release %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sink saw %v, want %v", got, want)
		}
	}
}

// TestRecordSyntheticModifier_AnnouncesNothingWithNoSinkWired covers the daemon
// before the tap exists, and every build with no daemon at all. Announcing to
// nobody is what the injection path did before this seam existed, so it stays a
// working injection rather than a refusal — and unwiring has to actually
// release the sink, or a torn-down tap keeps being told about injections.
func TestRecordSyntheticModifier_AnnouncesNothingWithNoSinkWired(t *testing.T) {
	sink := &fakeSink{}
	wireSink(t, sink)

	SetSyntheticModifierSink(nil)
	recordSyntheticModifier(action.ModShift, true)

	if got := sink.events(); len(got) != 0 {
		t.Fatalf("the unwired sink was still told about %v", got)
	}
}
