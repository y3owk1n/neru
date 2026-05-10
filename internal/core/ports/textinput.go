package ports

import "context"

// TextInputCallbacks holds callbacks for the native text input.
type TextInputCallbacks struct {
	OnQueryChanged func(query string)
	OnConfirm      func()
	OnCancel       func()
}

// TextInputFrame defines the bounding box for the text input.
type TextInputFrame struct {
	X      int
	Y      int
	Width  int
	Height int
}

// TextInputPort defines the interface for native text input capabilities.
type TextInputPort interface {
	StartHintSearchSession(
		ctx context.Context,
		callbacks TextInputCallbacks,
		frame TextInputFrame,
	) (bool, error)
	StopHintSearchSession(ctx context.Context) error
}
