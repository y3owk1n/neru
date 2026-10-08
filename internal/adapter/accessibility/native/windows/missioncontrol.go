//go:build windows

package windows

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// MissionControlClickableElements returns CodeNotSupported, since Mission
// Control is macOS-only.
func MissionControlClickableElements(
	_ context.Context,
	_ *zap.Logger,
	_ config.Provider,
	_ int,
) ([]*TreeNode, error) {
	return nil, derrors.New(derrors.CodeNotSupported, "Mission Control is macOS-only")
}
