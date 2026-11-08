//go:build !darwin && !linux && !windows

package vision

// readFrame is empty. This slot captures nothing.
type readFrame = struct{}

func retainFrame(frame readFrame) readFrame { return frame }

func releaseFrame(readFrame) {}
