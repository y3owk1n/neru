//go:build !darwin && !linux

package modes

import "github.com/y3owk1n/neru/internal/derrors"

func (h *Handler) showMonitorSelectLocked() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"monitor_select overlay is only supported on darwin",
	)
}

func (h *Handler) hideMonitorSelectLocked() error {
	return nil
}
