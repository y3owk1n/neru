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
// calling goroutine services the CoreFoundation main run loop, and returns the
// exit code testMain produced.
//
// Native work Neru depends on — building the keyboard layout maps, creating a
// CGEventTap — is dispatched to the main queue, which only drains while the main
// run loop runs. The daemon starts that loop (see cmd/neru); a `go test` binary
// never does. Without this helper those dispatches are never serviced:
// dispatch_async silently times out, leaving an empty keymap so every key name
// fails to parse, and dispatch_sync deadlocks. Whether a given test hit it
// depended on which OS thread the scheduler picked, which is what made the macOS
// integration tests flaky.
//
// Call it from TestMain, and lock the main thread from an init function in the
// same package so TestMain is guaranteed to run on it:
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
