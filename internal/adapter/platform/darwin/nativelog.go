//go:build darwin

package darwin

/*
#include "nativelog.h"
*/
import "C"

import (
	"fmt"
	"os"

	"go.uber.org/zap"
)

// Native log levels, matching NeruLogLevel in nativelog.h.
const (
	nativeLogDebug = 0
	nativeLogInfo  = 1
	nativeLogWarn  = 2
)

//export neruNativeLog
func neruNativeLog(level C.int, message, detail *C.char) {
	msg := C.GoString(message)

	var fields []zap.Field
	if detail != nil {
		fields = append(fields, zap.String("detail", C.GoString(detail)))
	}

	logger := Logger()
	if logger == nil {
		// Startup runs some native code before the daemon's logger exists, and
		// stderr is what reaches the user there.
		if level >= nativeLogWarn {
			if detail != nil {
				fmt.Fprintf(os.Stderr, "neru: %s: %s\n", msg, C.GoString(detail))
			} else {
				fmt.Fprintf(os.Stderr, "neru: %s\n", msg)
			}
		}

		return
	}

	switch level {
	case nativeLogDebug:
		logger.Debug(msg, fields...)
	case nativeLogInfo:
		logger.Info(msg, fields...)
	case nativeLogWarn:
		logger.Warn(msg, fields...)
	default:
		logger.Error(msg, fields...)
	}
}
