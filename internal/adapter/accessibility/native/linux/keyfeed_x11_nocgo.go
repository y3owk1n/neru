//go:build linux && !cgo

package linux

import (
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

func x11FeedKey(_ action.Modifiers, _ uint32) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"X11 key feeding requires CGO-enabled Linux builds",
	)
}
