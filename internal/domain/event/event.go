package event

import "image"

// Name names one kind of event.
type Name string

const (
	// ModeEnter is a mode becoming the active one. Mode names it.
	ModeEnter Name = "mode_enter"
	// ModeExit is the active mode closing. Mode and Reason say which and why,
	// and Action names the action a completed mode ran, when it ran one.
	ModeExit Name = "mode_exit"
	// Select is a step the user took moving a mode's target: a hint label, a
	// grid cell or subgrid key, a recursive grid level, a bisect cut. Opening,
	// refreshing or moving a mode publishes none. Mode and Point say which mode
	// and where the target is now. Action names what the mode's --action ran
	// there, and is empty when it ran none.
	Select Name = "select"
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
	// ScrollInvert is scroll inversion switching. On says which way.
	ScrollInvert Name = "scroll_invert"
	// ScreenShareHide is hiding the overlay from screen shares switching. On
	// says which way.
	ScreenShareHide Name = "screen_share_hide"
	// CursorSave is a cursor position saved. Slot and Point say where.
	CursorSave Name = "cursor_save"
	// CursorRestore is a saved cursor position taken for a restore, which
	// empties its slot. Slot names it.
	CursorRestore Name = "cursor_restore"
	// StickyModifiers is the set of sticky modifiers changing. Modifiers is the
	// set now, empty when none is held.
	StickyModifiers Name = "sticky_modifiers"
	// MonitorMove is the cursor moving to another display through
	// move_monitor or monitor_select. Monitor names the display.
	MonitorMove Name = "monitor_move"
	// ScreenChange is the displays changing, such as on a dock, an undock or a
	// wake.
	ScreenChange Name = "screen_change"
	// Ready is the daemon having started and taking hotkeys.
	Ready Name = "ready"
	// Quit is the daemon starting to shut down.
	Quit Name = "quit"
)

// All returns every event name, in the order the hooks table lists them.
func All() []Name {
	return []Name{
		ModeEnter,
		ModeExit,
		Select,
		AppFocus,
		Enable,
		Disable,
		ConfigReload,
		MissionControlActivated,
		MissionControlDeactivated,
		ScrollInvert,
		ScreenShareHide,
		CursorSave,
		CursorRestore,
		StickyModifiers,
		MonitorMove,
		ScreenChange,
		Ready,
		Quit,
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

	Name      Name
	Mode      string
	Reason    ExitReason
	Action    string
	BundleID  string
	OK        bool
	On        bool
	Slot      string
	Point     image.Point
	Modifiers string
	Monitor   string
}
