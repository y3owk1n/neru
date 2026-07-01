//go:build !darwin

package modes

import derrors "github.com/y3owk1n/neru/internal/core/errors"

func (h *Handler) showMonitorSelectLocked() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"monitor_select overlay is only supported on darwin",
	)
}

func (h *Handler) hideMonitorSelectLocked() error {
	return nil
}
