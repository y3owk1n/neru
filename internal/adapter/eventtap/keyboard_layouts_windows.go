//go:build windows

package eventtap

import winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"

func keyboardLayouts() ([]string, string) {
	return winplatform.KeyboardLayouts()
}
