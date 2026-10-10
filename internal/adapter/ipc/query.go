package ipc

// DisplayData is one display as `neru query displays` reports it. Bounds are
// in the coordinates `neru action move_mouse` takes.
type DisplayData struct {
	Name   string  `json:"name"`
	X      int     `json:"x"`
	Y      int     `json:"y"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Scale  float64 `json:"scale"`
}

// DisplaysData is the payload of `neru query displays`, in the platform's
// enumeration order.
type DisplaysData struct {
	Displays []DisplayData `json:"displays"`
}

// CursorData is the payload of `neru query cursor`. Display and DisplayIndex
// name the display holding the cursor, as `neru query displays` lists it, and
// are null when no display does. The index tells apart two displays that share
// a name.
type CursorData struct {
	X            int     `json:"x"`
	Y            int     `json:"y"`
	Display      *string `json:"display"`
	DisplayIndex *int    `json:"display_index"` //nolint:tagliatelle // snake_case like the status keys.
}

// WindowData is the payload of `neru query window`, in the coordinates `neru
// action move_mouse` takes. The payload is null when no window has focus.
type WindowData struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// AppData is the payload of `neru query app`.
type AppData struct {
	// BundleID is the string `bundle_id` matches in per-app config.
	BundleID string `json:"bundle_id"` //nolint:tagliatelle // snake_case like the status keys.
}

// HintData is one element hints mode would label, as `neru query hints`
// reports it. Title, Description and Value are the strings --text matches
// against, and Role is the native role spelled as --role takes it, such as
// ax:AXButton.
type HintData struct {
	Role        string `json:"role"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Value       string `json:"value"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// HintsData is the payload of `neru query hints`: the elements hints mode
// would label on the active screen.
type HintsData struct {
	Hints []HintData `json:"hints"`
}
