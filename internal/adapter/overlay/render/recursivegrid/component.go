package recursivegrid

import (
	"image"
)

// VirtualPointerState describes the recursive-grid virtual pointer state.
type VirtualPointerState struct {
	Visible   bool
	Position  image.Point
	Size      int
	FillColor string
	Char      string
	FontName  string
}
