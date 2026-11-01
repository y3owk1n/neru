package client

// SnapData is the payload of `neru action snap`: where the cursor landed and
// the element it landed on. X and Y are the landing point, in the coordinates
// `neru action move_mouse` takes. Role is the native role spelled as --role
// takes it, such as ax:AXButton, and Source is what found the element, as
// --strategy names it. Bounds is the element's whole extent, which can reach
// past the region the snap searched.
type SnapData struct {
	X      int        `json:"x"`
	Y      int        `json:"y"`
	Role   string     `json:"role"`
	Source string     `json:"source"`
	Bounds SnapBounds `json:"bounds"`
}

// SnapBounds is the extent of the element a snap landed on.
type SnapBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
