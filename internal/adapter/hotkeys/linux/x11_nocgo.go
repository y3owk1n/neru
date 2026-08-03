//go:build linux && !cgo

package linux

import (
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

func (m *Manager) registerX11Hotkey(id ports.HotkeyID, keyString string) error {
	_, _ = id, keyString
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 global hotkeys require CGO-enabled Linux builds",
	)
}

func (m *Manager) unregisterX11Hotkey(id ports.HotkeyID) {
	_ = id
}

func (m *Manager) unregisterAllX11Hotkeys() {}
