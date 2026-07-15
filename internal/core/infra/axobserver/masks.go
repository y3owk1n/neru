package axobserver

// Mask is a set of AX notifications to observe. Its bits are mapped to the
// platform's native notification set by the Platform implementation.
type Mask uint32

const (
	notifCreated Mask = 1 << iota
	notifUIDestroyed
	notifLayoutChanged
	notifWindowCreated
	notifWindowMoved
	notifWindowResized
	notifLoadComplete
	notifMenuOpened
	notifMenuClosed
	notifFocusedUIChanged
	notifValueChanged
	notifLiveRegionChanged
	notifLiveRegionCreated
	notifExpandedChanged
	notifRowExpanded
	notifRowCollapsed
	notifElementBusyChanged
)

// DefaultMask is the fixed set of notifications the observer watches. It covers
// the structural changes that mean the UI actually changed — an element or
// window appeared, moved, or vanished, a page finished loading, a menu opened,
// or focus moved — plus the signals browsers post for web content, where
// Chromium and Firefox emit no plain "created" notification (a live region
// updating, a disclosure or row expanding or collapsing, a busy flag clearing).
//
// It omits value_changed, which fires on every value update — a ticking clock, a
// progress bar, a live counter — and would wake the observer continuously.
const DefaultMask = notifCreated | notifUIDestroyed | notifLayoutChanged |
	notifWindowCreated | notifWindowMoved | notifWindowResized |
	notifLoadComplete | notifMenuOpened | notifMenuClosed |
	notifFocusedUIChanged |
	notifLiveRegionChanged | notifLiveRegionCreated | notifExpandedChanged |
	notifRowExpanded | notifRowCollapsed | notifElementBusyChanged

// highestBit is the top defined notification bit. Iterating from 1<<0 up to it
// walks every bit in the vocabulary, so a drift guard can check each one maps to
// a native notification without a separate list to keep in sync.
const highestBit = notifElementBusyChanged
