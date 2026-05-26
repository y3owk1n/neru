package hint

import "errors"

// ErrExternalMuNotHeld indicates a Manager method that invokes onUpdate
// synchronously was called without holding the external mutex passed to
// NewManager.
var ErrExternalMuNotHeld = errors.New("caller must hold externalMu")
