package client

// StatusData is the payload of `neru status`, and the status a `watch` stream
// opens with. Its fields are declared in key order, so it encodes byte for byte
// as the map it replaced.
//
// Capabilities and Profile gain entries as Neru adds subsystems, which is why
// docs/reference/ipc.md leaves their contents out of the contract.
type StatusData struct {
	BisectEnabled bool           `json:"bisect_enabled"` //nolint:tagliatelle // snake_case like the status keys.
	Capabilities  map[string]any `json:"capabilities"`
	Config        string         `json:"config"`
	// CursorFollowSelection is null when no mode carries the preference, which
	// is a different answer from a running mode not following the selection.
	CursorFollowSelection *bool                     `json:"cursor_follow_selection"` //nolint:tagliatelle // snake_case like the status keys.
	Enabled               bool                      `json:"enabled"`
	GridEnabled           bool                      `json:"grid_enabled"`            //nolint:tagliatelle // snake_case like the status keys.
	HiddenForScreenShare  bool                      `json:"hidden_for_screen_share"` //nolint:tagliatelle // snake_case like the status keys.
	HintsEnabled          bool                      `json:"hints_enabled"`           //nolint:tagliatelle // snake_case like the status keys.
	Mode                  string                    `json:"mode"`
	Profile               map[string]any            `json:"profile"`
	RecursiveGridEnabled  bool                      `json:"recursive_grid_enabled"` //nolint:tagliatelle // snake_case like the status keys.
	SavedCursorSlots      map[string]CursorSlotData `json:"saved_cursor_slots"`     //nolint:tagliatelle // snake_case like the status keys.
	ScrollInverted        bool                      `json:"scroll_inverted"`        //nolint:tagliatelle // snake_case like the status keys.
	StickyModifiers       string                    `json:"sticky_modifiers"`       //nolint:tagliatelle // snake_case like the status keys.
}

// CursorSlotData is one saved cursor position.
type CursorSlotData struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ScrollInvertData is the payload of `toggle-scroll-invert`.
type ScrollInvertData struct {
	Inverted bool `json:"inverted"`
}

// ScreenShareData is the payload of `toggle-screen-share`.
type ScreenShareData struct {
	Hidden bool `json:"hidden"`
}

// FollowSelectionData is the payload of `toggle-cursor-follow-selection`.
type FollowSelectionData struct {
	Following bool `json:"following"`
}

// WatchLine is one line of a `watch` stream after its first reply: the status
// snapshot it opens with, or one event. A line sets only the fields its event
// carries, and a snapshot that replaces missed events counts them in Dropped.
type WatchLine struct {
	Seq       uint64      `json:"seq"`
	Event     string      `json:"event"`
	Mode      string      `json:"mode,omitempty"`
	Reason    string      `json:"reason,omitempty"`
	Action    string      `json:"action,omitempty"`
	BundleID  string      `json:"bundle_id,omitempty"` //nolint:tagliatelle // snake_case like the status keys.
	OK        *bool       `json:"ok,omitempty"`
	On        *bool       `json:"on,omitempty"`
	Slot      string      `json:"slot,omitempty"`
	X         *int        `json:"x,omitempty"`
	Y         *int        `json:"y,omitempty"`
	Modifiers *string     `json:"modifiers,omitempty"`
	Monitor   string      `json:"monitor,omitempty"`
	Dropped   uint64      `json:"dropped,omitempty"`
	Status    *StatusData `json:"status,omitempty"`
}
