//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework Carbon -framework CoreGraphics -framework ApplicationServices -framework UserNotifications
*/
import "C"
