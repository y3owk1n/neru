package overlay

import (
	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// The overlay vocabulary lives in the manager package so the per-platform
// backends can satisfy the contract without importing this one, which selects
// them. These aliases keep the name the application layer already uses:
// overlay.Mode and overlay.ManagerInterface are what the modes, services and
// mode handler say, and there is no reason for a package split beneath them to
// reach up and rename anything.
type (
	// ManagerInterface is the overlay window management contract.
	ManagerInterface = manager.Interface
	// Mode is the overlay's current mode.
	Mode = manager.Mode
	// StateChange reports a mode transition.
	StateChange = manager.StateChange
	// MonitorSelectStyle styles the monitor-select overlay.
	MonitorSelectStyle = manager.MonitorSelectStyle
	// MonitorSelectTarget is one labeled monitor.
	MonitorSelectTarget = manager.MonitorSelectTarget
	// MonitorSelector is the optional capability a backend declares when it
	// can draw the monitor picker.
	MonitorSelector = manager.MonitorSelector
	// CapabilityReporter reports a manager's runtime support state.
	CapabilityReporter = manager.CapabilityReporter
	// HeadlessReporter declares a manager that has no surface to render on.
	HeadlessReporter = manager.HeadlessReporter
	// KeyboardCaptureController is the optional capability a backend declares
	// when its surface can hold or release the keyboard.
	KeyboardCaptureController = manager.KeyboardCaptureController
)

// Overlay modes.
const (
	ModeIdle          = manager.ModeIdle
	ModeHints         = manager.ModeHints
	ModeGrid          = manager.ModeGrid
	ModeScroll        = manager.ModeScroll
	ModeRecursiveGrid = manager.ModeRecursiveGrid
	ModeMonitorSelect = manager.ModeMonitorSelect
)
