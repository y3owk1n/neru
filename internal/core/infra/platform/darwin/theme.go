//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

#include "theme.h"
*/
import "C"

import (
	"sync"
)

// IsDarkMode returns true if macOS Dark Mode is currently active.
func IsDarkMode() bool {
	return C.isDarkMode() != 0
}

var (
	themeHandler   func(bool)
	themeHandlerMu sync.RWMutex
)

// SetThemeChangeHandler sets the callback function to be called when the system theme changes.
func SetThemeChangeHandler(handler func(bool)) {
	themeHandlerMu.Lock()
	defer themeHandlerMu.Unlock()
	themeHandler = handler
}

// StartThemeObserver starts observing macOS theme changes.
func StartThemeObserver() {
	C.startThemeObserver()
}

// StopThemeObserver stops observing macOS theme changes.
func StopThemeObserver() {
	C.stopThemeObserver()
}

//export handleThemeChanged
func handleThemeChanged(isDark C.int) {
	themeHandlerMu.RLock()
	handler := themeHandler
	themeHandlerMu.RUnlock()

	if handler != nil {
		go handler(isDark != 0)
	}
}
