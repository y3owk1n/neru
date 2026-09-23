//go:build linux && !cgo

package linux

import "testing"

func TestEventTap_SetKeyboardLayout_RejectsExplicitLayoutsWithoutCgo(t *testing.T) {
	t.Parallel()

	var eventTap EventTap
	if !eventTap.SetKeyboardLayout("") {
		t.Fatal("automatic fallback must remain available")
	}
	for _, layout := range []string{"first", "current", "com.apple.keylayout.US"} {
		if eventTap.SetKeyboardLayout(layout) {
			t.Errorf("layout %q was accepted without a native translation backend", layout)
		}
	}
}
