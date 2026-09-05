//go:build linux

package linux

import (
	"path/filepath"
	"strings"
)

// sysfsRoot is where the kernel describes input devices to userspace.
const sysfsRoot = "/sys"

// isVirtualInputNode reports whether an event node under /dev/input belongs to
// a uinput device, which is what a key remapper's output keyboard is. The
// kernel registers a uinput device with no parent, so sysfs lists it directly
// under devices/virtual/input; a keyboard on a bus sits under that bus, and a
// Bluetooth one under virtual/misc/uhid, so neither matches. root is the sysfs
// mount, a parameter so a test can lay out a sysfs of its own.
func isVirtualInputNode(root string, path string) bool {
	target, err := filepath.EvalSymlinks(filepath.Join(root, "class", "input", filepath.Base(path)))
	if err != nil {
		return false
	}

	return strings.HasPrefix(
		target,
		filepath.Join(root, "devices", "virtual", "input")+string(filepath.Separator),
	)
}
