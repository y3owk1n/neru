//go:build linux && cgo

package linux

import "testing"

func TestReferenceKeyboardLayout_Set(t *testing.T) {
	t.Parallel()

	var layout referenceKeyboardLayout
	if layout.current.Load() {
		t.Fatal("the default must use the first layout before configuration")
	}

	for _, test := range []struct {
		id      string
		wantOK  bool
		current bool
	}{
		{id: keyboardLayoutCurrent, wantOK: true, current: true},
		{id: "first", wantOK: true},
		{id: keyboardLayoutCurrent, wantOK: true, current: true},
		{id: "", wantOK: true},
		{id: keyboardLayoutCurrent, wantOK: true, current: true},
		{id: "com.apple.keylayout.US"},
	} {
		if got := layout.set(test.id); got != test.wantOK {
			t.Errorf("set(%q) = %v, want %v", test.id, got, test.wantOK)
		}

		if got := layout.current.Load(); got != test.current {
			t.Errorf("set(%q): current = %v, want %v", test.id, got, test.current)
		}
	}
}
