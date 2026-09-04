//go:build windows

package keyfeed

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/windows"
)

// postKey injects an already-normalized key into the focused application via
// SendInput. The events are tagged as Neru's own so the daemon's keyboard hook
// ignores them.
func postKey(normalized string) error {
	return windows.FeedKey(normalized)
}
