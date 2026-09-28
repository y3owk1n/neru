//go:build linux

package eventtap

import (
	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/ports"
)

func keyboardLayouts() ports.KeyboardLayouts {
	layouts := eventtaplinux.ListKeyboardLayouts()

	return ports.KeyboardLayouts{
		Names:     layouts.Names,
		Reference: layouts.Reference,
		Unmatched: layouts.Unmatched,
	}
}
