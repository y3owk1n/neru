//go:build linux

package systray

import "github.com/y3owk1n/neru/internal/adapter/systray/linux"

// menuItem aliases the backend's menu item so the shared adapter in adapter.go
// can name it without knowing which platform it came from.
type menuItem = linux.MenuItem

// The rest forwards the backend's process-wide tray API. Each backend wraps a
// single native tray, so these are functions rather than methods.
var (
	setTitle        = linux.SetTitle
	setTooltip      = linux.SetTooltip
	setTemplateIcon = linux.SetTemplateIcon
	addMenuItem     = linux.AddMenuItem
	addSeparator    = linux.AddSeparator
)

// Run starts the tray event loop and blocks until Quit.
func Run(onReady, onExit func()) { linux.Run(onReady, onExit) }

// RunHeadless starts the tray without a visible icon.
func RunHeadless(onReady, onExit func()) { linux.RunHeadless(onReady, onExit) }

// Quit stops the tray event loop.
func Quit() { linux.Quit() }

// AddMenuItem appends a top-level item and returns the backend's item.
//
// It is exported because systray_test.go exercises the live backend's menu
// tree — one test, three backends, whichever one this build selected.
func AddMenuItem(title string) *menuItem { return addMenuItem(title) }

// ResetForTesting clears the tray's process-wide state between tests.
func ResetForTesting() { linux.ResetForTesting() }
