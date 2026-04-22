//go:build linux

//nolint:testpackage // These tests validate unexported evdev translation helpers directly.
package eventtap

import "testing"

func TestEvdevModifierName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code uint16
		want string
	}{
		{code: evdevKeyLeftShift, want: "shift"},
		{code: evdevKeyRightCtrl, want: "ctrl"},
		{code: evdevKeyLeftAlt, want: "alt"},
		{code: evdevKeyRightMeta, want: "cmd"},
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
		{code: evdevKeyEnter, want: "Return"},
		{code: evdevKeyBackspace, want: "Backspace"},
		{code: evdevKeyLeft, want: "Left"},
		{code: evdevKeyF1, want: "F1"},
	}

	for _, testCase := range testCases {
		if got := evdevKeyName(testCase.code); got != testCase.want {
			t.Fatalf("evdevKeyName(%d) = %q, want %q", testCase.code, got, testCase.want)
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

	var got []string

	eventTap := NewEventTap(func(key string) {
		got = append(got, key)
	}, nil)

	state := waylandEvdevKeyState{
		pressed: make(map[uint16]bool),
	}

	eventTap.handleWaylandEvdevEvent(&state, waylandEvdevEvent{
		eventType: evdevEventKey,
		code:      evdevKeyU,
		value:     evdevValueRepeat,
	})

	if len(got) != 0 {
		t.Fatalf("got keys %v, want none", got)
	}
}

func TestHandleWaylandEvdevEvent_AllowsRepeatAfterPress(t *testing.T) {
	t.Parallel()

	var got []string

	eventTap := NewEventTap(func(key string) {
		got = append(got, key)
	}, nil)

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

	if len(got) != 2 || got[0] != "u" || got[1] != "u" {
		t.Fatalf("got keys %v, want [u u]", got)
	}
}
