package ports

import (
	"context"
	"image"
)

// TextInputCallbacks holds callbacks for the native text input.
type TextInputCallbacks struct {
	OnQueryChanged func(query string)
	OnConfirm      func()
	OnCancel       func()
}

// TextInputFrame defines the bounding box for the text input. X and Y are
// relative to Screen, the active screen in global coordinates, which is where
// the overlay draws the search box.
type TextInputFrame struct {
	X      int
	Y      int
	Width  int
	Height int
	Screen image.Rectangle
}

// TextInputPort is the interface for native text input capabilities.
type TextInputPort interface {
	StartHintSearchSession(
		ctx context.Context,
		callbacks TextInputCallbacks,
		frame TextInputFrame,
	) (bool, error)
	StopHintSearchSession(ctx context.Context) error
}
