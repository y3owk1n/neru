//go:build windows

package eventtap

import (
	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/ports"
)

func keyboardLayouts() ports.KeyboardLayouts {
	names, reference, unmatched := winplatform.KeyboardLayouts()

	return ports.KeyboardLayouts{Names: names, Reference: reference, Unmatched: unmatched}
}
