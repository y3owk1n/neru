//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=14.0
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework Carbon -framework CoreGraphics -framework ApplicationServices -framework UserNotifications -framework QuartzCore -framework Vision -framework ScreenCaptureKit
*/
import "C"
