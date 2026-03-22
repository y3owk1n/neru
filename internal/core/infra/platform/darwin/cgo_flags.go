//go:build darwin

package darwin

/*
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework Carbon -framework CoreGraphics -framework ApplicationServices -framework UserNotifications
*/
import "C"
