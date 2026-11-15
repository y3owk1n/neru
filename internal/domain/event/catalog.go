package event

import "strconv"

// HookPrefix turns an event name into its hook key in [hooks] (ADR 0008).
const HookPrefix = "on_"

// Variable is one environment variable an exec step run for an event can
// see. Value says what it holds, for a reader.
type Variable struct {
	Name  string
	Value string
	read  func(Event) (string, bool)
}

// Spec is one event as a user meets it: the hook it runs, when, and the
// variables that hook's exec steps can see. A variable listed here may still
// be unset for a particular event, as its Value says.
type Spec struct {
	Name        Name
	Description string
	Variables   []Variable
}

// Hook is the [hooks] key that runs for the event.
func (s Spec) Hook() string { return HookPrefix + string(s.Name) }

// HookVariables returns every variable the event's hook can see, NERU_EVENT
// first.
func (s Spec) HookVariables() []Variable {
	return append([]Variable{varEvent}, s.Variables...)
}

// The variables, in the order the hooks reference lists them. NERU_EVENT is
// set for every event and so is not listed against any one of them.
var (
	varEvent = Variable{
		Name:  "NERU_EVENT",
		Value: "The event, such as `mode_enter`",
		read:  func(evt Event) (string, bool) { return string(evt.Name), true },
	}
	varMode = Variable{
		Name:  "NERU_MODE",
		Value: "The mode, named as `neru status` names it",
		read:  func(evt Event) (string, bool) { return evt.Mode, evt.Mode != "" },
	}
	varReason = Variable{
		Name: "NERU_REASON",
		Value: "`completed` after a selection, `switched` when another mode opens in its place, " +
			"else `canceled`",
		read: func(evt Event) (string, bool) { return string(evt.Reason), evt.Reason != "" },
	}
	varAction = Variable{
		Name: "NERU_ACTION",
		Value: "The action the selection ran, such as `left_click`, or `left_click,left_click` for " +
			"a chain, or `snap` for a snap. `on_mode_exit` sees it only when `NERU_REASON` is " +
			"`completed`. Unset when it ran none, such as for a binding with no `--action`",
		read: func(evt Event) (string, bool) { return evt.Action, evt.Action != "" },
	}
	varBundleID = Variable{
		Name:  "NERU_BUNDLE_ID",
		Value: "The application, as `bundle_id` names it",
		read:  func(evt Event) (string, bool) { return evt.BundleID, evt.BundleID != "" },
	}
	varOK = Variable{
		Name:  "NERU_OK",
		Value: "`true` or `false`",
		read:  func(evt Event) (string, bool) { return strconv.FormatBool(evt.OK), true },
	}
	varOn = Variable{
		Name:  "NERU_ON",
		Value: "`true` when switched on, else `false`",
		read:  func(evt Event) (string, bool) { return strconv.FormatBool(evt.On), true },
	}
	varSlot = Variable{
		Name:  "NERU_SLOT",
		Value: "The cursor slot, `default` when none was named",
		read:  func(evt Event) (string, bool) { return evt.Slot, evt.Slot != "" },
	}
	varX = Variable{
		Name:  "NERU_X",
		Value: "The x of the position saved, or of the point selected",
		read:  func(evt Event) (string, bool) { return strconv.Itoa(evt.Point.X), true },
	}
	varY = Variable{
		Name:  "NERU_Y",
		Value: "The y of the position saved, or of the point selected",
		read:  func(evt Event) (string, bool) { return strconv.Itoa(evt.Point.Y), true },
	}
	// Set even when empty, so a hook reads a release as no modifiers.
	varModifiers = Variable{
		Name: "NERU_MODIFIERS",
		Value: "The set held now, such as `cmd,shift`, in the spelling `--modifier` takes. " +
			"Empty when none is",
		read: func(evt Event) (string, bool) { return evt.Modifiers, true },
	}
	varMonitor = Variable{
		Name:  "NERU_MONITOR",
		Value: "The display, as `move_monitor --name` takes it",
		read:  func(evt Event) (string, bool) { return evt.Monitor, evt.Monitor != "" },
	}
)

// catalog is every event, in the order the hooks reference lists them.
var catalog = []Spec{
	{
		Name:        ModeEnter,
		Description: "A mode opens",
		Variables:   []Variable{varMode},
	},
	{
		Name:        ModeExit,
		Description: "A mode closes, including on the way into another one",
		Variables:   []Variable{varMode, varReason, varAction},
	},
	{
		Name: Select,
		Description: "A step you take moves a mode's target, such as a hint label, a grid cell or " +
			"a recursive grid level, or a `snap` moves the cursor",
		Variables: []Variable{varMode, varAction, varX, varY},
	},
	{
		Name:        AppFocus,
		Description: "Another application comes to the front",
		Variables:   []Variable{varBundleID},
	},
	{Name: Enable, Description: "Neru resumes, such as on `neru start`"},
	{Name: Disable, Description: "Neru pauses, such as on `neru stop`"},
	{
		Name:        ConfigReload,
		Description: "A config reload finishes, whether or not the new file loaded",
		Variables:   []Variable{varOK},
	},
	{
		Name:        MissionControlActivated,
		Description: "Mission Control opens. macOS only, needs `hints.detect_mission_control`",
	},
	{
		Name:        MissionControlDeactivated,
		Description: "Mission Control closes. macOS only, needs `hints.detect_mission_control`",
	},
	{
		Name:        ScrollInvert,
		Description: "Scroll inversion switches on or off",
		Variables:   []Variable{varOn},
	},
	{
		Name: ScreenShareHide,
		Description: "`toggle-screen-share` switches on or off. macOS and Windows only, since " +
			"nothing hides the overlay on Linux",
		Variables: []Variable{varOn},
	},
	{
		Name:        CursorSave,
		Description: "`save_cursor_pos` saves a position",
		Variables:   []Variable{varSlot, varX, varY},
	},
	{
		Name:        CursorRestore,
		Description: "`restore_cursor_pos` takes a saved position, which empties its slot",
		Variables:   []Variable{varSlot},
	},
	{
		Name:        StickyModifiers,
		Description: "A sticky modifier is armed or released. Needs `sticky_modifiers.enabled`",
		Variables:   []Variable{varModifiers},
	},
	{
		Name:        MonitorMove,
		Description: "`move_monitor` or `monitor_select` moves the cursor to another display",
		Variables:   []Variable{varMonitor},
	},
	{
		Name:        ScreenChange,
		Description: "The displays change, such as on a dock, an undock or a wake",
	},
	{Name: Ready, Description: "The daemon has started and takes hotkeys"},
	{Name: Quit, Description: "The daemon starts to shut down. Runs `exec` steps only"},
}

// Catalog returns every event, in the order the hooks reference lists them.
func Catalog() []Spec {
	specs := make([]Spec, len(catalog))
	copy(specs, catalog)

	return specs
}

// Variables returns every variable an exec step run for an event can see,
// NERU_EVENT first, in the order the hooks reference lists them.
func Variables() []Variable {
	return []Variable{
		varEvent, varMode, varReason, varAction, varBundleID, varOK, varOn,
		varSlot, varX, varY, varModifiers, varMonitor,
	}
}

// lookup returns the spec for name, and false for a name no event has.
func lookup(name Name) (Spec, bool) {
	for _, spec := range catalog {
		if spec.Name == name {
			return spec, true
		}
	}

	return Spec{}, false
}
