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
