package ports

// ObservationSource identifies which hint source a process was resolved from.
// It selects the set of accessibility notifications worth observing on that
// process (e.g. a menubar target only cares about menu open/close, while a
// window cares about layout and window geometry).
type ObservationSource int

const (
	// ObservationFrontWindow is the frontmost application's focused window.
	ObservationFrontWindow ObservationSource = iota
	// ObservationAppMenubar is the frontmost application's menu bar (same pid as
	// the front window).
	ObservationAppMenubar
	// ObservationAdditionalMenubar is a user-configured menu-extra / status-item
	// process.
	ObservationAdditionalMenubar
	// ObservationNotificationCenter is com.apple.notificationcenterui.
	ObservationNotificationCenter
	// ObservationDock is com.apple.dock.
	ObservationDock
	// ObservationStageManager is com.apple.WindowManager.
	ObservationStageManager
	// ObservationPIP is com.apple.PIPAgent.
	ObservationPIP
	// ObservationScreenCapture is com.apple.screencaptureui.
	ObservationScreenCapture
)

// ObservationTarget is one process the hint scan resolved, to be watched for
// accessibility changes while hints mode is active. Only the pid is needed to
// register an observer (it attaches to the application element, whose descendant
// notifications propagate up); BundleID guards against a recycled pid attaching
// to the wrong process, and Source selects the notification set.
type ObservationTarget struct {
	PID      int
	BundleID string
	Source   ObservationSource
}
