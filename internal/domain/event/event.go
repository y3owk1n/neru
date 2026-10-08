package event

// Name names one kind of event.
type Name string

const (
	// ModeEnter is a mode becoming the active one. Mode names it.
	ModeEnter Name = "mode_enter"
	// ModeExit is the active mode closing. Mode and Reason say which and why.
	ModeExit Name = "mode_exit"
	// AppFocus is another application coming to the front. BundleID names it.
	AppFocus Name = "app_focus"
	// Enable is Neru resuming after a pause.
	Enable Name = "enable"
	// Disable is Neru pausing.
	Disable Name = "disable"
	// ConfigReload is a configuration reload finishing. OK says whether it
	// took.
	ConfigReload Name = "config_reload"
	// MissionControlActivated is Mission Control opening. macOS only.
	MissionControlActivated Name = "mission_control_activated"
	// MissionControlDeactivated is Mission Control closing. macOS only.
	MissionControlDeactivated Name = "mission_control_deactivated"
)

// All returns every event name, in the order the hooks table lists them.
func All() []Name {
	return []Name{
		ModeEnter,
		ModeExit,
		AppFocus,
		Enable,
		Disable,
		ConfigReload,
		MissionControlActivated,
		MissionControlDeactivated,
	}
}

// ExitReason says why a mode closed.
type ExitReason string

const (
	// ExitCompleted is a mode closing because the user selected something.
	ExitCompleted ExitReason = "completed"
	// ExitCancelled is a mode closing without a selection, such as on Escape.
	ExitCancelled ExitReason = "canceled"
	// ExitSwitched is a mode closing because another one is opening.
	ExitSwitched ExitReason = "switched"
)

// Event is one thing that happened. Only the fields its Name documents are
// set.
type Event struct {
	// Seq numbers events in the order the bus published them, starting at 1.
	Seq uint64
	// Dropped counts the events this subscriber missed just before this one
	// because it was not keeping up. Anything it derived from the events it
	// has seen may be stale.
	Dropped uint64

	Name     Name
	Mode     string
	Reason   ExitReason
	BundleID string
	OK       bool
}
