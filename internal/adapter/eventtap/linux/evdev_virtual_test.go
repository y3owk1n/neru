//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"testing"
)

// A sysfs of the test's own: one keyboard on a USB bus, one uinput keyboard,
// and one Bluetooth keyboard, which the kernel also files under virtual but as
// a uhid child, laid out the way the kernel lays them out.
func TestIsVirtualInputNode_OnlyAUinputDeviceIsVirtual(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	devices := map[string]string{
		"event3": "devices/pci0000:00/0000:00:04.0/usb1/1-3/1-3:1.0/0003:0627:0001.0003/input/input3/event3",
		"event5": "devices/virtual/input/input12/event5",
		"event7": "devices/virtual/misc/uhid/0005:046D:B33B.0004/input/input25/event7",
	}

	class := filepath.Join(root, "class", "input")

	err := os.MkdirAll(class, 0o750)
	if err != nil {
		t.Fatal(err)
	}

	for node, target := range devices {
		err = os.MkdirAll(filepath.Join(root, target), 0o750)
		if err != nil {
			t.Fatal(err)
		}

		err = os.Symlink(filepath.Join(root, target), filepath.Join(class, node))
		if err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name    string
		node    string
		virtual bool
	}{
		{name: "a USB keyboard", node: "/dev/input/event3", virtual: false},
		{name: "a uinput keyboard", node: "/dev/input/event5", virtual: true},
		{name: "a Bluetooth keyboard", node: "/dev/input/event7", virtual: false},
		{name: "a node sysfs does not list", node: "/dev/input/event9", virtual: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := isVirtualInputNode(root, testCase.node); got != testCase.virtual {
				t.Errorf("isVirtualInputNode = %v, want %v", got, testCase.virtual)
			}
		})
	}
}
