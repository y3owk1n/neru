//go:build integration && darwin

package darwin_test

import (
	"os"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// Pin the main thread during package init so TestMain still runs on it.
func init() {
	runtime.LockOSThread()
}

// TestMain services the macOS main run loop while the tests run. Building the
// keyboard layout maps happens on the main queue, which nothing drains in a
// plain `go test` binary (see runloop.go).
func TestMain(m *testing.M) {
	os.Exit(darwin.RunMainLoopForTesting(m.Run))
}

// Virtual key codes under test, mirroring the KeyCode enum in keymap.h.
const (
	keyCodeReturn       = 36 // main-keyboard Return
	keyCodeDelete       = 51 // main-keyboard Delete (backspace position)
	keyCodeNumpadDot    = 65
	keyCodeNumpadClear  = 71
	keyCodeNumpadEnter  = 76
	keyCodeNumpadEquals = 81
	keyCodeNumpad0      = 82
	keyCodeNumpad5      = 87
	keyCodeNumpad9      = 92
	keyCodeNumpadDivide = 75
	keyCodeNumpadMinus  = 78
	keyCodeNumpadPlus   = 69
	keyCodeNumpadStar   = 67
)

// Named-key spellings the event tap emits on the wire.
const (
	namedKeyReturn = "Return"
	namedKeyClear  = "Clear"
)

// TestKeymap_KeyCodeToCharacter_NumpadFoldsToNamedKeys pins issue #1372. The
// numpad keys with a named-key equivalent must fold to it, the way the Wayland
// keymap folds KP_Enter -> Return, instead of surfacing raw control characters
// ("\x03" for numpad Enter, "\x7f" for numpad Clear) that either match no
// binding or impersonate the Delete key.
func TestKeymap_KeyCodeToCharacter_NumpadFoldsToNamedKeys(t *testing.T) {
	tests := []struct {
		name    string
		keyCode uint16
		want    string
	}{
		{"numpad Enter folds to Return", keyCodeNumpadEnter, namedKeyReturn},
		{"numpad Clear folds to Clear", keyCodeNumpadClear, namedKeyClear},
		{"numpad digit 0 stays a digit", keyCodeNumpad0, "0"},
		{"numpad digit 5 stays a digit", keyCodeNumpad5, "5"},
		{"numpad digit 9 stays a digit", keyCodeNumpad9, "9"},
		{"numpad dot stays a dot", keyCodeNumpadDot, "."},
		{"numpad plus stays an operator", keyCodeNumpadPlus, "+"},
		{"main-keyboard Return keeps its carriage-return character", keyCodeReturn, "\r"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := darwin.KeyCodeToCharacter(testCase.keyCode, 0)
			if got != testCase.want {
				t.Errorf(
					"KeyCodeToCharacter(%d, 0) = %q, want %q",
					testCase.keyCode,
					got,
					testCase.want,
				)
			}
		})
	}
}

// TestKeymap_KeyCodeToName_NumpadFoldsToNamedKeys pins the modifier-combo
// path of issue #1372. The event tap resolves "Cmd+..." / "Shift+..." combos
// through the keycode-to-name map, so numpad Enter must name "Return" there
// too or Cmd+NumpadEnter degrades to a bare character event.
func TestKeymap_KeyCodeToName_NumpadFoldsToNamedKeys(t *testing.T) {
	tests := []struct {
		name    string
		keyCode uint16
		want    string
	}{
		{"numpad Enter names Return", keyCodeNumpadEnter, namedKeyReturn},
		{"numpad Clear names Clear", keyCodeNumpadClear, namedKeyClear},
		{"main-keyboard Return still names Return", keyCodeReturn, namedKeyReturn},
		{"main-keyboard Delete still names Delete", keyCodeDelete, "Delete"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := darwin.KeyCodeToName(testCase.keyCode)
			if got != testCase.want {
				t.Errorf("KeyCodeToName(%d) = %q, want %q", testCase.keyCode, got, testCase.want)
			}
		})
	}
}

// TestKeymap_KeyCodeToCharacter_NumpadEmitsNoControlCharacters pins the
// acceptance criterion that no raw control character reaches key comparison
// from the numpad path.
func TestKeymap_KeyCodeToCharacter_NumpadEmitsNoControlCharacters(t *testing.T) {
	numpadKeyCodes := []uint16{
		keyCodeNumpadDot, keyCodeNumpadStar, keyCodeNumpadPlus, keyCodeNumpadClear,
		keyCodeNumpadDivide, keyCodeNumpadEnter, keyCodeNumpadMinus, keyCodeNumpadEquals,
		keyCodeNumpad0, 83, 84, 85, 86, keyCodeNumpad5, 88, 89, 91, keyCodeNumpad9,
	}

	for _, keyCode := range numpadKeyCodes {
		got := darwin.KeyCodeToCharacter(keyCode, 0)
		for _, r := range got {
			if r < 0x20 || r == 0x7f {
				t.Errorf(
					"KeyCodeToCharacter(%d, 0) = %q contains control character %U",
					keyCode, got, r,
				)
			}
		}
	}
}

// TestKeyboardLayouts_ListsTheEnabledLayoutsAndTheReference pins what neru
// doctor lists on macOS: the enabled keyboard layouts by input source ID, as
// general.kb_layout_to_use takes them, with the one keys are named in among
// them.
func TestKeyboardLayouts_ListsTheEnabledLayoutsAndTheReference(t *testing.T) {
	names, reference, _ := darwin.KeyboardLayouts()
	if len(names) == 0 {
		t.Fatal("KeyboardLayouts listed no enabled keyboard layout")
	}

	if !slices.Contains(names, reference) {
		t.Fatalf("reference layout %q is not among the enabled layouts %v", reference, names)
	}
}

// TestSetReferenceKeyboardLayout_LeavesTheApplyQueuedWhenTheMainQueueIsBlocked
// pins that a caller off the main thread never applies the layout itself when
// the main queue stalls. Resolving the layout calls TIS, which must run on the
// main thread, so the apply waits in the queue and runs once the main thread
// is free. The call returns true meanwhile, because a layout not yet applied
// has not failed to resolve. An ID no layout has makes the apply visible on
// any machine, since it flips the unmatched flag KeyboardLayouts reports.
func TestSetReferenceKeyboardLayout_LeavesTheApplyQueuedWhenTheMainQueueIsBlocked(t *testing.T) {
	const missingLayout = "com.example.keylayout.NoSuchLayout"

	tests := []struct {
		name  string
		prior string
		next  string
	}{
		{name: "missing layout after auto", prior: "", next: missingLayout},
		{name: "auto after missing layout", prior: missingLayout, next: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(func() { darwin.SetReferenceKeyboardLayout("") })

			darwin.SetReferenceKeyboardLayout(test.prior)

			_, _, priorUnmatched := darwin.KeyboardLayouts()
			if priorUnmatched != (test.prior != "") {
				t.Fatalf("unmatched is %v after setting %q", priorUnmatched, test.prior)
			}

			release := darwin.BlockMainQueueForTesting()
			t.Cleanup(release)

			// The call gives the main queue 500ms, so taking this long means
			// it waited on the blocked queue.
			const callLimit = 2 * time.Second

			start := time.Now()

			if !darwin.SetReferenceKeyboardLayout(test.next) {
				t.Fatal("SetReferenceKeyboardLayout reported a layout not yet applied as missing")
			}

			if elapsed := time.Since(start); elapsed >= callLimit {
				t.Fatalf("SetReferenceKeyboardLayout waited %v for the blocked main queue", elapsed)
			}

			if _, _, unmatched := darwin.KeyboardLayouts(); unmatched != priorUnmatched {
				t.Fatal("layout was applied off the main thread while the main queue was blocked")
			}

			release()

			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, _, unmatched := darwin.KeyboardLayouts(); unmatched != priorUnmatched {
					return
				}

				if time.Now().After(deadline) {
					t.Fatal("queued layout was never applied after the main queue drained")
				}

				time.Sleep(50 * time.Millisecond)
			}
		})
	}
}

// TestKeyboardLayouts_ReadsNothingOffTheMainThreadWhenTheMainQueueIsBlocked
// pins that a caller off the main thread never reads the layouts through TIS
// itself, which must run on the main thread. neru doctor and the health check
// call it from the IPC goroutine. While the main queue is blocked it returns no
// names and no reference, which doctor shows as not read yet. The unmatched
// flag is the one the resolver last recorded. Only a TIS call produces the
// names, so an empty list shows none ran here. Once the test releases the
// queue, KeyboardLayouts reads the layouts again.
func TestKeyboardLayouts_ReadsNothingOffTheMainThreadWhenTheMainQueueIsBlocked(t *testing.T) {
	_, _, wantUnmatched := darwin.KeyboardLayouts()

	release := darwin.BlockMainQueueForTesting()
	t.Cleanup(release)

	type layouts struct {
		names     []string
		reference string
		unmatched bool
	}

	result := make(chan layouts, 1)

	go func() {
		names, reference, unmatched := darwin.KeyboardLayouts()
		result <- layouts{names: names, reference: reference, unmatched: unmatched}
	}()

	var blocked layouts

	select {
	case blocked = <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("KeyboardLayouts waited for the blocked main queue")
	}

	if len(blocked.names) != 0 || blocked.reference != "" {
		t.Fatalf(
			"KeyboardLayouts read %v and %q off the main thread",
			blocked.names,
			blocked.reference,
		)
	}

	if blocked.unmatched != wantUnmatched {
		t.Fatalf(
			"unmatched is %v while the main queue is blocked, want %v",
			blocked.unmatched,
			wantUnmatched,
		)
	}

	release()

	names, reference, _ := darwin.KeyboardLayouts()
	if len(names) == 0 {
		t.Fatal("KeyboardLayouts read nothing after the main queue was released")
	}

	if !slices.Contains(names, reference) {
		t.Fatalf("reference layout %q is not among the enabled layouts %v", reference, names)
	}
}
