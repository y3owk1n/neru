//go:build linux

package linux

import (
	"path/filepath"
	"strings"
)

// sysfsRoot is where the kernel describes input devices to userspace.
const sysfsRoot = "/sys"

// isVirtualInputNode reports whether a /dev/input event node belongs to a
// uinput device, as a remapper's output keyboard does. uinput devices have no
// parent, so sysfs lists them under devices/virtual/input, while bus and
// Bluetooth (virtual/misc/uhid) keyboards sit elsewhere. root is the sysfs
// mount, a test seam.
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
