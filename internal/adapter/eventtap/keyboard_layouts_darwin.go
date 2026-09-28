//go:build darwin

package eventtap

import "github.com/y3owk1n/neru/internal/adapter/platform/darwin"

func keyboardLayouts() ([]string, string) {
	return darwin.KeyboardLayouts()
}
