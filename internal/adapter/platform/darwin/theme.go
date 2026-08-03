//go:build darwin

package darwin

/*
#include "theme.h"
*/
import "C"

// IsDarkMode returns true if macOS Dark Mode is currently active.
func IsDarkMode() bool {
	return C.NeruIsDarkMode() != 0
}

var themeHandlerSlot cgoSlot[func(bool)]

// SetThemeChangeHandler sets the callback function to be called when the system theme changes.
func SetThemeChangeHandler(handler func(bool)) {
	themeHandlerSlot.Set(handler)
}

// StartThemeObserver starts observing macOS theme changes.
func StartThemeObserver() {
	C.NeruStartThemeObserver()
}

// StopThemeObserver stops observing macOS theme changes.
func StopThemeObserver() {
	C.NeruStopThemeObserver()
}

//export handleThemeChanged
func handleThemeChanged(isDark C.int) {
	dark := isDark != 0
	themeHandlerSlot.withValidAsync(func(handler func(bool)) {
		handler(dark)
	})
}
