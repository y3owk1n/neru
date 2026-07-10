package axobserver

import (
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/core/infra/axnotify"
)

// Mask is a set of AX notifications to observe. Its bits are mapped to the
// platform's native notification set by the Platform implementation. Every valid
// notification name in the axnotify vocabulary has exactly one bit here.
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

// notificationBits maps each supported notification name to its mask bit. The
// keys are the axnotify name constants, so this map and the axnotify vocabulary
// cannot drift apart; TestNotificationBitsCoverVocabulary pins that every name
// has exactly one entry here.
var notificationBits = map[string]Mask{
	axnotify.Created:                 notifCreated,
	axnotify.UIDestroyed:             notifUIDestroyed,
	axnotify.LayoutChanged:           notifLayoutChanged,
	axnotify.WindowCreated:           notifWindowCreated,
	axnotify.WindowMoved:             notifWindowMoved,
	axnotify.WindowResized:           notifWindowResized,
	axnotify.LoadComplete:            notifLoadComplete,
	axnotify.MenuOpened:              notifMenuOpened,
	axnotify.MenuClosed:              notifMenuClosed,
	axnotify.FocusedUIElementChanged: notifFocusedUIChanged,
	axnotify.ValueChanged:            notifValueChanged,
	axnotify.LiveRegionChanged:       notifLiveRegionChanged,
	axnotify.LiveRegionCreated:       notifLiveRegionCreated,
	axnotify.ExpandedChanged:         notifExpandedChanged,
	axnotify.RowExpanded:             notifRowExpanded,
	axnotify.RowCollapsed:            notifRowCollapsed,
	axnotify.ElementBusyChanged:      notifElementBusyChanged,
}

// MaskFromNames builds a Mask from user-supplied notification names (see
// axnotify.Names for the valid set). The first unknown name is returned as an
// error that lists the supported set, so a typo in config fails loudly instead
// of quietly observing the wrong thing. An empty list yields a zero Mask and no
// error; callers that require at least one notification enforce that themselves
// (see the config validator).
func MaskFromNames(names []string) (Mask, error) {
	var mask Mask

	for _, name := range names {
		bit, ok := notificationBits[name]
		if !ok {
			return 0, fmt.Errorf(
				"unknown notification %q: supported names are %s",
				name,
				strings.Join(axnotify.AllNames(), ", "),
			)
		}

		mask |= bit
	}

	return mask, nil
}
