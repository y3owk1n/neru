//go:build darwin

package darwin

/*
#include <CoreFoundation/CoreFoundation.h>
#include <dispatch/dispatch.h>

// Runs the main run loop for at most `seconds` and returns the CFRunLoopRunResult.
static int neruRunMainLoopSlice(double seconds) {
	return (int)CFRunLoopRunInMode(kCFRunLoopDefaultMode, seconds, false);
}

static void neruStopMainLoop(void) {
	CFRunLoopStop(CFRunLoopGetMain());
}

// Queues a block that holds the main thread until neruReleaseMainQueue is
// called with the returned handle, or `limitSeconds` pass. The handle owns a +1
// reference that neruReleaseMainQueue takes back, so call it exactly once.
static void *neruBlockMainQueue(double limitSeconds) {
	dispatch_semaphore_t released = dispatch_semaphore_create(0);
	dispatch_async(dispatch_get_main_queue(), ^{
		dispatch_semaphore_wait(released, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(limitSeconds * NSEC_PER_SEC)));
	});
	return (__bridge_retained void *)released;
}

static void neruReleaseMainQueue(void *handle) {
	dispatch_semaphore_t released = (__bridge_transfer dispatch_semaphore_t)handle;
	dispatch_semaphore_signal(released);
}
*/
import "C"

import (
	"sync"
	"time"
)

// mainLoopSlice bounds how long a single run loop pump blocks before the
// caller re-checks whether the tests finished.
const mainLoopSlice = 100 * time.Millisecond

// blockMainQueueLimit bounds how long BlockMainQueueForTesting holds the main
// thread when a failed test never releases it.
const blockMainQueueLimit = 30 * time.Second

// cfRunLoopRunFinished mirrors kCFRunLoopRunFinished. The mode had nothing left
// to service, so CFRunLoopRunInMode returned without waiting out its timeout.
const cfRunLoopRunFinished = 1

// RunMainLoopForTesting runs testMain on a background goroutine while the
// caller services the CoreFoundation main run loop. Native work (keymaps,
// CGEventTap) is dispatched to the main queue, which only drains while that
// loop runs. The daemon starts it and a `go test` binary never does, so without
// this dispatch_async times out and dispatch_sync deadlocks. That was the
// source of the old macOS test flakes. Use from TestMain, with the main thread locked
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

		// A run loop with no sources left returns immediately, so sleep out
		// the slice instead of spinning on it.
		if C.neruRunMainLoopSlice(C.double(mainLoopSlice.Seconds())) == cfRunLoopRunFinished {
			time.Sleep(mainLoopSlice)
		}
	}
}

// BlockMainQueueForTesting queues work that holds the main thread until the
// returned function is called, so a test can see what a caller off the main
// thread does while the main queue stalls. It returns at once, and work queued
// after it runs once released. The release function is safe to call more than
// once, so a test can call it and also register it with t.Cleanup. The main thread is
// freed after blockMainQueueLimit even if release is never called.
func BlockMainQueueForTesting() func() {
	handle := C.neruBlockMainQueue(C.double(blockMainQueueLimit.Seconds()))

	var once sync.Once

	return func() { once.Do(func() { C.neruReleaseMainQueue(handle) }) }
}
