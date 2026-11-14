//go:build linux

package linux

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
	"github.com/y3owk1n/neru/internal/derrors"
)

// MissionControlClickableElements returns CodeNotSupported, since Mission
// Control is macOS-only.
func MissionControlClickableElements(
	_ context.Context,
	_ *zap.Logger,
	_ ax.SettingsSource,
	_ int,
) ([]*TreeNode, error) {
	return nil, derrors.New(derrors.CodeNotSupported, "Mission Control is macOS-only")
}
