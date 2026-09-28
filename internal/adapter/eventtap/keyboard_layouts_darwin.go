//go:build darwin

package eventtap

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/ports"
)

func keyboardLayouts() ports.KeyboardLayouts {
	names, reference, unmatched := darwin.KeyboardLayouts()

	return ports.KeyboardLayouts{Names: names, Reference: reference, Unmatched: unmatched}
}
