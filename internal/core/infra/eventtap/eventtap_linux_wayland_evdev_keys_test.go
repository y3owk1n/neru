//go:build linux

//nolint:testpackage // These tests validate unexported evdev translation helpers directly.
package eventtap

import (
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
		{code: evdevKeyEnter, want: evdevKeyNameReturn},
		{code: evdevKeyBackspace, want: "Backspace"},
		{code: evdevKeyLeft, want: evdevKeyNameLeft},
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
