// Package axnotify defines the vocabulary of accessibility notification names
// that hints auto-refresh can observe. It is the single source of truth for
// which names are valid, shared by the config layer (which validates the user's
// allowed_notifications list) and the observer service (which maps each name to
// a native notification bit). It depends on nothing outside the standard
// library, so both layers can import it without an import cycle.
package axnotify

import "sort"

// The supported notification names.
//
// The default set names a structural change — an element or window appeared,
// moved, or vanished, a page finished loading, a menu opened, or focus moved.
// These are safe to watch continuously: they fire on real navigation, not on
// idle churn.
const (
	Created                 = "created"
	UIDestroyed             = "ui_destroyed"
	LayoutChanged           = "layout_changed"
	WindowCreated           = "window_created"
	WindowMoved             = "window_moved"
	WindowResized           = "window_resized"
	LoadComplete            = "load_complete"
	MenuOpened              = "menu_opened"
	MenuClosed              = "menu_closed"
	FocusedUIElementChanged = "focused_ui_element_changed"

	// The following fire for browser web content, where Chromium and Firefox emit
	// no plain "element created" notification. A live region updating, a
	// disclosure or row expanding or collapsing, and a busy flag clearing are the
	// signals those engines do post when a page's content changes.
	LiveRegionChanged  = "live_region_changed"
	LiveRegionCreated  = "live_region_created"
	ExpandedChanged    = "expanded_changed"
	RowExpanded        = "row_expanded"
	RowCollapsed       = "row_collapsed"
	ElementBusyChanged = "element_busy_changed"
)

// ValueChanged is a valid notification name but is not in the default set. A
// value change fires on every value update — a ticking clock, a progress bar, a
// live counter — so watching it can wake the observer continuously. It is
// available for apps whose content updates in place without a structural change,
// but a user must opt into it explicitly in allowed_notifications.
const ValueChanged = "value_changed"

// defaultNames lists the notifications watched by default, once each. Names
// derives its sorted output from this, and it is what the shipped config's
// allowed_notifications enumerates.
var defaultNames = []string{
	Created,
	UIDestroyed,
	LayoutChanged,
	WindowCreated,
	WindowMoved,
	WindowResized,
	LoadComplete,
	MenuOpened,
	MenuClosed,
	FocusedUIElementChanged,
	LiveRegionChanged,
	LiveRegionCreated,
	ExpandedChanged,
	RowExpanded,
	RowCollapsed,
	ElementBusyChanged,
}

// optionalNames lists valid names that are off by default because they are noisy.
var optionalNames = []string{
	ValueChanged,
}

// Names returns the default notification names in sorted order — the set the
// shipped config enables. Config defaults draw from this so the template and the
// observer cannot drift.
func Names() []string {
	return sortedCopy(defaultNames)
}

// AllNames returns every valid notification name in sorted order, including the
// opt-in ones. Validation and the observer's name-to-bit map draw from this so a
// user may name any valid notification, not only the defaults.
func AllNames() []string {
	all := make([]string, 0, len(defaultNames)+len(optionalNames))
	all = append(all, defaultNames...)
	all = append(all, optionalNames...)

	return sortedCopy(all)
}

// IsName reports whether name is a valid notification name (default or opt-in).
func IsName(name string) bool {
	for _, candidate := range defaultNames {
		if candidate == name {
			return true
		}
	}

	for _, candidate := range optionalNames {
		if candidate == name {
			return true
		}
	}

	return false
}

func sortedCopy(names []string) []string {
	out := make([]string, len(names))
	copy(out, names)

	sort.Strings(out)

	return out
}
