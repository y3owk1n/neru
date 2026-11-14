//go:build !linux

package accessibility

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
	"github.com/y3owk1n/neru/internal/adapter/accessibility/native"
)

// NewPlatformAXClient returns the native client: AXUIElement on macOS, UI
// Automation on Windows. Both are the same shell over a build-tagged element
// and tree layer, so one factory covers them.
func NewPlatformAXClient(logger *zap.Logger, settingsSource ax.SettingsSource) ax.Client {
	return native.New(logger, settingsSource)
}
