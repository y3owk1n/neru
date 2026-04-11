//go:build linux && !cgo

package hotkeys

import derrors "github.com/y3owk1n/neru/internal/core/errors"

func (m *Manager) registerX11Hotkey(id HotkeyID, keyString string) error {
	_, _ = id, keyString
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 global hotkeys require CGO-enabled Linux builds",
	)
}

func (m *Manager) unregisterX11Hotkey(id HotkeyID) {
	_ = id
}

func (m *Manager) unregisterAllX11Hotkeys() {}
