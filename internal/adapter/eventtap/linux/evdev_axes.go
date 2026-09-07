//go:build linux

package linux

// The absolute axes that make a node a touch surface or a tablet rather than
// a keyboard: the position of a finger or a pen. A keyboard with a trackpad
// built in advertises them, and grabbing it for its keys would swallow its
// touches, since re-emitting those would need the device's own axis ranges and
// slots. Any other absolute axis is left to count for nothing: a Bluetooth
// keyboard's consumer-control page puts a volume knob on ABS_VOLUME, and a
// keyboard is no less a keyboard for having one.
const (
	evdevAbsX           = 0x00
	evdevAbsY           = 0x01
	evdevAbsMtSlot      = 0x2f
	evdevAbsMtPositionX = 0x35
	evdevAbsMtPositionY = 0x36
)

// evdevPointerAxes is the mask of those axes over the EVIOCGBIT(EV_ABS)
// bitmap, which is one word: ABS_CNT is 64.
const evdevPointerAxes uint64 = 1<<evdevAbsX | 1<<evdevAbsY | 1<<evdevAbsMtSlot |
	1<<evdevAbsMtPositionX | 1<<evdevAbsMtPositionY

// hasPointerAxes reports whether an EVIOCGBIT(EV_ABS) bitmap carries a
// position axis, which is what marks a touch surface or a tablet.
func hasPointerAxes(bits uint64) bool { return bits&evdevPointerAxes != 0 }
