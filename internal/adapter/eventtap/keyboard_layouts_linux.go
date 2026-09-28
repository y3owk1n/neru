//go:build linux

package eventtap

import eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"

func keyboardLayouts() ([]string, string) {
	layouts := eventtaplinux.ListKeyboardLayouts()

	return layouts.Names, layouts.Reference
}
