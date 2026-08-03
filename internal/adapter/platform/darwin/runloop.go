//go:build darwin

package darwin

/*
#include <CoreFoundation/CoreFoundation.h>

// Runs the main run loop for at most `seconds` and returns the CFRunLoopRunResult.
static int neruRunMainLoopSlice(double seconds) {
	return (int)CFRunLoopRunInMode(kCFRunLoopDefaultMode, seconds, false);
}

static void neruStopMainLoop(void) {
	CFRunLoopStop(CFRunLoopGetMain());
}
*/
import "C"

import "time"

// mainLoopSlice bounds how long a single run loop pump blocks before the
// caller re-checks whether the tests finished.
const mainLoopSlice = 100 * time.Millisecond

// cfRunLoopRunFinished mirrors kCFRunLoopRunFinished: the mode had nothing left
// to service, so CFRunLoopRunInMode returned without waiting out its timeout.
const cfRunLoopRunFinished = 1

// RunMainLoopForTesting runs testMain on a background goroutine while the
// caller services the CoreFoundation main run loop. Native work (keymaps,
// CGEventTap) is dispatched to the main queue, which only drains while that
// loop runs; the daemon starts it, a `go test` binary never does, so without
// this dispatch_async times out and dispatch_sync deadlocks — the source of
// the old macOS test flakes. Use from TestMain, with the main thread locked
// from an init in the same package:
//
//	func init() { runtime.LockOSThread() }
//
//	func TestMain(m *testing.M) { os.Exit(darwin.RunMainLoopForTesting(m.Run)) }
func RunMainLoopForTesting(testMain func() int) int {
	result := make(chan int, 1)

	go func() {
		result <- testMain()

		// Cut the current pump short so the exit code is picked up
		// immediately instead of after the remainder of the slice.
		C.neruStopMainLoop()
	}()

	for {
		select {
		case code := <-result:
			return code
		default:
		}

		// A run loop with no sources left returns immediately; sleep out
		// the slice instead of spinning on it.
		if C.neruRunMainLoopSlice(C.double(mainLoopSlice.Seconds())) == cfRunLoopRunFinished {
			time.Sleep(mainLoopSlice)
		}
	}
}
