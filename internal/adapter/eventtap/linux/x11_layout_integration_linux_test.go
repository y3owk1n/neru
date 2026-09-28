//go:build integration && linux && cgo

package linux

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

// XKB names of the keys the X11 tests press: the key left of W, and the right
// Ctrl key, which grp:rctrl_switch turns into a layout switch while held.
const (
	x11KeyQ         = "AD01"
	x11KeyRightCtrl = "RCTL"
)

// TestEventTap_X11_NamesKeysInTheReferenceLayout pins which physical key an X11
// binding matches while another language is active. It is the key that types
// the binding in the active layout when that layout is ASCII-capable, and in the
// first ASCII-capable layout of the server's keymap otherwise. When no layout
// is ASCII-capable, the tap names keys in the active layout.
//
// It loads keymaps into the running X server with setxkbmap and restores the
// original afterwards, so it runs only in the desktop tier. grp:rctrl_switch
// makes holding the right Ctrl key select the second layout, as a user
// switching language would.
func TestEventTap_X11_NamesKeysInTheReferenceLayout(t *testing.T) {
	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}

	if os.Getenv("DISPLAY") == "" || platform.DetectLinuxBackend().IsWayland() {
		t.Skip("skipping: no X11 session; this drives the X11 keyboard grab")
	}

	_, err := exec.LookPath("setxkbmap")
	if err != nil {
		t.Skip("skipping: setxkbmap is not installed to load a test keymap")
	}

	restoreX11Keymap(t)

	for _, test := range []struct {
		name    string
		layout  string
		variant string
		options string
		keys    []string
		want    string
	}{
		{
			name:   "a Latin first layout names keys while it is active",
			layout: testXkbLatinThenCyrillic,
			keys:   []string{x11KeyQ},
			want:   "q",
		},
		{
			name:   "switching to a non-Latin layout leaves the binding on its key",
			layout: testXkbLatinThenCyrillic,
			keys:   []string{x11KeyRightCtrl, x11KeyQ},
			want:   "q",
		},
		{
			name:   "a non-Latin first layout yields to the first Latin one",
			layout: "ru,us",
			keys:   []string{x11KeyQ},
			want:   "q",
		},
		{
			name:    "an active Latin layout names keys itself",
			layout:  "us,us",
			variant: ",dvorak",
			keys:    []string{x11KeyRightCtrl, x11KeyQ},
			want:    "'",
		},
		{
			name:   "Shift chooses the level inside the reference layout",
			layout: testXkbLatinThenCyrillic,
			keys:   []string{x11KeyRightCtrl, "LFSH", x11KeyQ},
			want:   "Shift+q",
		},
		{
			name:    "AltGr reaches the third level of the reference layout",
			layout:  "de,ru",
			options: "lv3:ralt_switch",
			keys:    []string{x11KeyRightCtrl, "RALT", x11KeyQ},
			want:    "@",
		},
		{
			name:    "a remapped modifier is identified in the active layout",
			layout:  testXkbLatinThenCyrillic,
			options: "ctrl:swapcaps",
			keys:    []string{x11KeyRightCtrl, "CAPS", x11KeyQ},
			want:    "Ctrl+q",
		},
		{
			name:   "a keymap with no Latin layout follows a switch to its second",
			layout: testXkbNoLatinLayout,
			keys:   []string{x11KeyRightCtrl, x11KeyQ},
			want:   ";",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := "grp:rctrl_switch"
			if test.options != "" {
				options += "," + test.options
			}

			setX11Keymap(t, test.layout, test.variant, options)

			keys := make(chan string, 64)
			eventTap := NewEventTap(func(key string) { keys <- key }, nil)

			eventTap.Enable()
			defer eventTap.Destroy()

			time.Sleep(x11GrabSettle)

			err := pressX11Keys(test.keys...)
			if err != nil {
				t.Fatalf("pressing %v on %s: %v", test.keys, test.layout, err)
			}

			var got []string

			deadline := time.After(x11KeysWait)
			for !slices.Contains(got, test.want) {
				select {
				case key := <-keys:
					got = append(got, key)
				case <-deadline:
					t.Fatalf("layouts %s, keys %v: dispatched %q, want %q among them",
						test.layout, test.keys, got, test.want)
				}
			}
		})
	}
}

// restoreX11Keymap puts the server's keymap back the way the test found it.
func restoreX11Keymap(t *testing.T) {
	t.Helper()

	query, err := exec.CommandContext(t.Context(), "setxkbmap", "-query").Output()
	if err != nil {
		t.Skipf("skipping: setxkbmap cannot read the current keymap: %v", err)
	}

	var layout, variant, options string

	for line := range strings.Lines(string(query)) {
		field, value, _ := strings.Cut(line, ":")
		switch strings.TrimSpace(field) {
		case "layout":
			layout = strings.TrimSpace(value)
		case "variant":
			variant = strings.TrimSpace(value)
		case "options":
			options = strings.TrimSpace(value)
		}
	}

	t.Cleanup(func() {
		// t.Context is already canceled when cleanups run.
		err := exec.CommandContext(context.Background(), "setxkbmap",
			"-layout", layout, "-variant", variant, "-option", "", "-option", options,
		).Run()
		if err != nil {
			t.Errorf("restoring the keymap %s: %v", layout, err)
		}
	})
}

func setX11Keymap(t *testing.T, layout, variant, options string) {
	t.Helper()

	err := exec.CommandContext(t.Context(), "setxkbmap",
		"-layout", layout, "-variant", variant, "-option", "", "-option", options,
	).Run()
	if err != nil {
		t.Fatalf("loading the keymap %s: %v", layout, err)
	}
}
