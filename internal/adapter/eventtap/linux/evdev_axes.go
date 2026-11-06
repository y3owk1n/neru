//go:build linux

package linux

// The absolute axes that mark a touch surface or tablet: finger or pen
// position. Grabbing a keyboard with a built-in trackpad would swallow its
// touches, since re-emitting them needs the device's own axis ranges and
// slots. Other absolute axes do not count, so a keyboard with a volume knob on
// ABS_VOLUME is still a keyboard.
const (
	evdevAbsX           = 0x00
	evdevAbsY           = 0x01
	evdevAbsMtSlot      = 0x2f
	evdevAbsMtPositionX = 0x35
	evdevAbsMtPositionY = 0x36
)

// evdevPointerAxes is the mask of those axes over the EVIOCGBIT(EV_ABS)
// bitmap, which is one word, since ABS_CNT is 64.
const evdevPointerAxes uint64 = 1<<evdevAbsX | 1<<evdevAbsY | 1<<evdevAbsMtSlot |
	1<<evdevAbsMtPositionX | 1<<evdevAbsMtPositionY

// hasPointerAxes reports whether an EVIOCGBIT(EV_ABS) bitmap carries a
// position axis, which is what marks a touch surface or a tablet.
func hasPointerAxes(bits uint64) bool { return bits&evdevPointerAxes != 0 }
