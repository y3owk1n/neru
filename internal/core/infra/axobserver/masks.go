package axobserver

import "github.com/y3owk1n/neru/internal/core/ports"

// Notification mask bits. These MUST match the enum in
// internal/core/infra/platform/darwin/axobserver.h and the mirrored constants in
// axobserver.go; the darwin build asserts equality in masks_darwin_test.go.
const (
	maskLayoutChanged           uint32 = 1 << 0
	maskCreated                 uint32 = 1 << 1
	maskUIElementDestroyed      uint32 = 1 << 2
	maskWindowCreated           uint32 = 1 << 3
	maskWindowMoved             uint32 = 1 << 4
	maskWindowResized           uint32 = 1 << 5
	maskFocusedUIElementChanged uint32 = 1 << 6
	maskMenuOpened              uint32 = 1 << 7
	maskMenuClosed              uint32 = 1 << 8
	maskValueChanged            uint32 = 1 << 9
)

// maskForSource returns the notification bits worth watching for a source. The
// front window watches structure and geometry; menu targets watch menu and
// element lifecycle only (so unrelated window churn does not fire refreshes);
// Notification Center watches window/element create and destroy. valueChanged is
// added only for the front window and only when explicitly enabled, since it is
// the noisiest notification.
func maskForSource(src ports.ObservationSource, watchValueChanged bool) uint32 {
	switch src {
	case ports.ObservationFrontWindow:
		m := maskLayoutChanged | maskCreated | maskUIElementDestroyed |
			maskWindowCreated | maskWindowMoved | maskWindowResized |
			maskFocusedUIElementChanged
		if watchValueChanged {
			m |= maskValueChanged
		}

		return m
	case ports.ObservationAppMenubar, ports.ObservationAdditionalMenubar:
		return maskMenuOpened | maskMenuClosed | maskCreated | maskUIElementDestroyed
	case ports.ObservationNotificationCenter:
		return maskWindowCreated | maskUIElementDestroyed
	case ports.ObservationDock, ports.ObservationStageManager:
		return maskCreated | maskUIElementDestroyed | maskLayoutChanged
	case ports.ObservationPIP, ports.ObservationScreenCapture:
		return maskCreated | maskUIElementDestroyed
	default:
		return maskCreated | maskUIElementDestroyed
	}
}
