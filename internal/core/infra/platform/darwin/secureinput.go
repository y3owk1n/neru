//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

#include "secureinput.h"
*/
import "C"

// IsSecureInputEnabled returns true if macOS secure input mode is currently active.
func IsSecureInputEnabled() bool {
	return C.isSecureInputEnabled() != 0
}

// ShowSecureInputNotification displays a notification about active secure input.
func ShowSecureInputNotification() {
	C.showSecureInputNotification()
}
