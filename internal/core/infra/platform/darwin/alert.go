//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

#include "alert.h"
*/
import "C"

import "unsafe"

// ShowConfigValidationError displays a native macOS alert for configuration validation errors.
func ShowConfigValidationError(errorMessage, configPath string) {
	cError := C.CString(errorMessage)
	cPath := C.CString(configPath)
	defer C.free(unsafe.Pointer(cError)) //nolint:nlreturn
	defer C.free(unsafe.Pointer(cPath))  //nolint:nlreturn

	C.showConfigValidationErrorAlert(cError, cPath)
}

// ShowNotification displays a native macOS notification with a title and message.
func ShowNotification(title, message string) {
	cTitle := C.CString(title)
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cTitle))   //nolint:nlreturn
	defer C.free(unsafe.Pointer(cMessage)) //nolint:nlreturn

	C.showNotification(cTitle, cMessage)
}
