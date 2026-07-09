//go:build darwin

package axobserver

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// TestMaskBitsMatchNativeHeader guards against drift between the mask bits in
// masks.go and the C enum in the darwin AXObserver header. The darwin package
// mirrors that enum directly from cgo (darwin.AXNotif* = C.NeruAXNotif*), so
// asserting equality here fails the build if the two ever disagree.
func TestMaskBitsMatchNativeHeader(t *testing.T) {
	cases := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"LayoutChanged", maskLayoutChanged, darwin.AXNotifLayoutChanged},
		{"Created", maskCreated, darwin.AXNotifCreated},
		{"UIElementDestroyed", maskUIElementDestroyed, darwin.AXNotifUIElementDestroyed},
		{"WindowCreated", maskWindowCreated, darwin.AXNotifWindowCreated},
		{"WindowMoved", maskWindowMoved, darwin.AXNotifWindowMoved},
		{"WindowResized", maskWindowResized, darwin.AXNotifWindowResized},
		{"FocusedUIElementChanged", maskFocusedUIElementChanged, darwin.AXNotifFocusedUIElementChanged},
		{"MenuOpened", maskMenuOpened, darwin.AXNotifMenuOpened},
		{"MenuClosed", maskMenuClosed, darwin.AXNotifMenuClosed},
		{"ValueChanged", maskValueChanged, darwin.AXNotifValueChanged},
		{"LoadComplete", maskLoadComplete, darwin.AXNotifLoadComplete},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("mask %s = %#x, native header = %#x", c.name, c.got, c.want)
		}
	}
}
