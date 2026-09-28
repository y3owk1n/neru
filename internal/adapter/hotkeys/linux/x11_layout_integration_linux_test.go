//go:build integration && linux && cgo

package linux

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/ports"
)

// TestManager_Register_GrabsTheKeyOfTheForcedX11Layout pins that a forced
// general.kb_layout_to_use decides which physical key an X11 [hotkeys] grab
// takes, as it decides which key a mode binding answers. With Dvorak forced, a
// hotkey written q grabs the key that types q in Dvorak.
//
// It loads keymaps into the running X server with setxkbmap and restores the
// original afterwards, so it runs only in the desktop tier.
func TestManager_Register_GrabsTheKeyOfTheForcedX11Layout(t *testing.T) {
	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}

	if os.Getenv("DISPLAY") == "" || platform.DetectLinuxBackend().IsWayland() {
		t.Skip("skipping: no X11 session; this resolves an X11 grab")
	}

	for _, tool := range []string{"setxkbmap", "xkbcomp"} {
		_, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("skipping: %s is not installed", tool)
		}
	}

	restoreX11Keymap(t)
	runSetxkbmap(t, "-layout", "us,us", "-variant", ",dvorak", "-option", "")

	var eventTap eventtaplinux.EventTap

	t.Cleanup(func() { eventTap.SetKeyboardLayout("") })

	for _, test := range []struct {
		name   string
		forced string
		key    string
	}{
		{name: "the automatic choice grabs the key of the first layout", key: "AD01"},
		{name: "a forced layout grabs its own key", forced: "English (Dvorak)", key: "AB02"},
		{name: "a forced layout the keymap lacks grabs as if none were", forced: "Klingon", key: "AD01"},
	} {
		t.Run(test.name, func(t *testing.T) {
			eventTap.SetKeyboardLayout(test.forced)

			manager := NewManager(nil)
			defer manager.UnregisterAll()

			hotkeyID, err := manager.Register("q", func() {})
			if err != nil {
				t.Fatalf("registering q with %q forced: %v", test.forced, err)
			}

			got := grabbedX11Keycode(t, manager, hotkeyID)
			if want := x11KeycodeNamed(t, test.key); got != want {
				t.Fatalf(
					"with %q forced, q grabs keycode %d, want %d (%s)",
					test.forced,
					got,
					want,
					test.key,
				)
			}
		})
	}
}

// grabbedX11Keycode is the keycode the manager grabbed for a hotkey.
func grabbedX11Keycode(t *testing.T, manager *Manager, hotkeyID ports.HotkeyID) int {
	t.Helper()

	stateAny, ok := x11States.Load(manager)
	if !ok {
		t.Fatal("the manager holds no X11 connection after registering")
	}

	state, _ := stateAny.(*x11HotkeyState)

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	return int(state.bindings[hotkeyID].keycode)
}

// x11KeycodeNamed reads the keycode of an XKB key name from the server's
// keymap, as xkbcomp prints it: a line like "<AB02> = 53;".
func x11KeycodeNamed(t *testing.T, name string) int {
	t.Helper()

	keymap, err := exec.CommandContext(t.Context(), "xkbcomp", "-xkb", os.Getenv("DISPLAY"), "-").
		Output()
	if err != nil {
		t.Fatalf("xkbcomp cannot print the keymap: %v", err)
	}

	for line := range strings.Lines(string(keymap)) {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "<"+name+"> = "); ok {
			keycode, err := strconv.Atoi(strings.TrimSuffix(value, ";"))
			if err == nil {
				return keycode
			}
		}
	}

	t.Fatalf("the keymap names no key %s", name)

	return 0
}

// restoreX11Keymap puts the server's keymap back the way the test found it.
func restoreX11Keymap(t *testing.T) {
	t.Helper()

	query, err := exec.CommandContext(t.Context(), "setxkbmap", "-query").Output()
	if err != nil {
		t.Skipf("skipping: setxkbmap cannot read the current keymap: %v", err)
	}

	args := []string{"-option", ""}

	for line := range strings.Lines(string(query)) {
		field, value, _ := strings.Cut(line, ":")
		switch field = strings.TrimSpace(field); field {
		case "layout", "variant", "option", "options":
			if field == "options" {
				field = "option"
			}

			args = append(args, "-"+field, strings.TrimSpace(value))
		}
	}

	t.Cleanup(func() {
		// t.Context is already canceled when cleanups run.
		err := exec.CommandContext(context.Background(), "setxkbmap", args...).Run()
		if err != nil {
			t.Errorf("restoring the keymap: %v", err)
		}
	})
}

func runSetxkbmap(t *testing.T, args ...string) {
	t.Helper()

	err := exec.CommandContext(t.Context(), "setxkbmap", args...).Run()
	if err != nil {
		t.Fatalf("setxkbmap %v: %v", args, err)
	}
}
