//go:build linux && cgo

//nolint:testpackage // These tests validate unexported evdev translation helpers directly.
package linux

import (
	"strconv"
	"testing"
	"time"
)

const asyncTimeout = time.Second

func TestEvdevModifierName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code uint16
		want string
	}{
		{code: evdevKeyLeftShift, want: evdevModifierShift},
		{code: evdevKeyRightCtrl, want: evdevModifierCtrl},
		{code: evdevKeyLeftAlt, want: evdevModifierAlt},
		{code: evdevKeyRightMeta, want: evdevModifierCmd},
		{code: evdevKeyA, want: ""},
	}

	for _, testCase := range testCases {
		if got := evdevModifierName(testCase.code); got != testCase.want {
			t.Fatalf("evdevModifierName(%d) = %q, want %q", testCase.code, got, testCase.want)
		}
	}
}

func TestEvdevKeyName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code uint16
		want string
	}{
		{code: evdevKeyA, want: "a"},
		{code: evdevKeySlash, want: "/"},
		{code: evdevKeyEnter, want: evdevKeyNameReturn},
		{code: evdevKeyBackspace, want: "Backspace"},
		{code: evdevKeyLeft, want: evdevKeyNameLeft},
		{code: evdevKeyF1, want: "F1"},
		{code: evdevKeyF12, want: "F12"},
		{code: evdevKeyF13, want: "F13"},
		{code: evdevKeyF20, want: "F20"},
		{code: evdevKeyF21, want: "F21"},
		{code: evdevKeyF24, want: "F24"},
	}

	for _, testCase := range testCases {
		if got := evdevKeyName(testCase.code); got != testCase.want {
			t.Fatalf("evdevKeyName(%d) = %q, want %q", testCase.code, got, testCase.want)
		}
	}
}

// TestEvdevKeyNameFunctionKeysContiguous pins the F13-F24 evdev codes, which
// are not adjacent to the F1-F12 block (KEY_F13 is 183, not 89).
func TestEvdevKeyNameFunctionKeysContiguous(t *testing.T) {
	t.Parallel()

	const firstHighFunctionKeyCode = 183

	for index := 13; index <= 24; index++ {
		code := uint16(firstHighFunctionKeyCode + index - 13)
		want := "F" + strconv.Itoa(index)

		if got := evdevKeyName(code); got != want {
			t.Errorf("evdevKeyName(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestEvdevModifierStatePrefix(t *testing.T) {
	t.Parallel()

	state := evdevModifierState{}
	state.update("ctrl", true)
	state.update("shift", true)
	state.update("cmd", true)

	if got := state.prefix(); got != "Shift+Ctrl+Cmd+" {
		t.Fatalf("prefix() = %q, want %q", got, "Shift+Ctrl+Cmd+")
	}

	state.update("ctrl", false)

	if got := state.prefix(); got != "Shift+Cmd+" {
		t.Fatalf("prefix() after ctrl release = %q, want %q", got, "Shift+Cmd+")
	}
}

func TestHandleWaylandEvdevEvent_IgnoresRepeatWithoutPress(t *testing.T) {
	t.Parallel()

	keyCh := make(chan string, 1)

	eventTap := NewEventTap(func(key string) {
		keyCh <- key
	}, nil)
	t.Cleanup(func() { eventTap.Destroy() })

	state := waylandEvdevKeyState{
		pressed: make(map[uint16]bool),
	}

	eventTap.handleWaylandEvdevEvent(&state, waylandEvdevEvent{
		eventType: evdevEventKey,
		code:      evdevKeyU,
		value:     evdevValueRepeat,
	})

	select {
	case <-keyCh:
		t.Fatal("expected no events, got one")
	case <-time.After(asyncTimeout):
	}
}

func TestHandleWaylandEvdevEvent_AllowsRepeatAfterPress(t *testing.T) {
	t.Parallel()

	keyCh := make(chan string, 2)

	eventTap := NewEventTap(func(key string) {
		keyCh <- key
	}, nil)
	t.Cleanup(func() { eventTap.Destroy() })

	state := waylandEvdevKeyState{
		pressed: make(map[uint16]bool),
	}

	eventTap.handleWaylandEvdevEvent(&state, waylandEvdevEvent{
		eventType: evdevEventKey,
		code:      evdevKeyU,
		value:     evdevValuePress,
	})
	eventTap.handleWaylandEvdevEvent(&state, waylandEvdevEvent{
		eventType: evdevEventKey,
		code:      evdevKeyU,
		value:     evdevValueRepeat,
	})

	var got1, got2 string

	select {
	case got1 = <-keyCh:
	case <-time.After(asyncTimeout):
		t.Fatal("timeout waiting for first key event")
	}

	select {
	case got2 = <-keyCh:
	case <-time.After(asyncTimeout):
		t.Fatal("timeout waiting for second key event")
	}

	if got1 != "u" || got2 != "u" {
		t.Fatalf("got keys [%s %s], want [u u]", got1, got2)
	}
}
