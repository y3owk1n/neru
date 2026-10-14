//go:build integration && linux && cgo

package linux

// A keyboard destroyed with a key down, against a real kernel. The kernel
// queues a release for every held key as it unregisters a device, but a read
// made after the device is gone fails before returning it. A reader frozen
// while the keyboard went away makes that read after the wake, so the capture
// sends the release itself. Desktop-safe: only the test's own uinput keyboard is
// read.

import (
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

// readerSettle is how long the reader is given to read a key that was just sent.
const readerSettle = 100 * time.Millisecond

func TestWaylandEvdevCapture_AKeyboardDestroyedWithAKeyDownReleasesIt(t *testing.T) {
	keyboard, err := createTestProxyKeyboard()
	if err != nil {
		t.Skipf("/dev/uinput is not writable here, so no keyboard can be created: %v", err)
	}

	var node *proxyNode

	deadline := time.Now().Add(proxyNodeAppearance)
	for node == nil && time.Now().Before(deadline) {
		node = keyboard.node()
		if node == nil {
			time.Sleep(waylandEvdevIdlePollInterval)
		}
	}

	if node == nil {
		keyboard.destroy()
		t.Fatal("the keyboard's event node did not appear")
	}

	capture := &waylandEvdevCapture{
		logger: zap.NewNop(),
		// Unbuffered, so the reader holds the press until the test takes it and
		// is not reading while the keyboard goes away.
		events: make(chan waylandEvdevEvent),
		stopCh: make(chan struct{}),
		// Marked as let go on purpose, so its reader does not rescan
		// /dev/input when it exits.
		devices: map[*os.File]trackedDevice{node.file: {released: true}},
		files:   []*os.File{node.file},
	}

	// Grab it as the proxy grabs its keyboards. The press, and the release the
	// kernel queues at unregister, then reach only this fd and never the
	// desktop session.
	grabErr := grabFile(node.file, true)
	if grabErr != nil {
		keyboard.destroy()
		t.Fatalf("grabbing the test keyboard: %v", grabErr)
	}

	capture.startReader(node.file)
	keyboard.key(evdevKeyLeftMeta, evdevValuePress)

	// Give the reader time to read the press and block handing it over. A
	// reader frozen by a suspend is not reading either.
	time.Sleep(readerSettle)
	keyboard.destroy()

	received := make(chan []waylandEvdevEvent)

	go func() {
		var events []waylandEvdevEvent
		for event := range capture.events {
			events = append(events, event)
		}

		received <- events
	}()

	exited := make(chan struct{})

	go func() {
		capture.done.Wait()
		close(exited)
	}()

	select {
	case <-exited:
	case <-time.After(proxyNodeAppearance):
		t.Fatal("the reader did not exit after its keyboard was destroyed")
	}

	close(capture.events)

	presses, releases := 0, 0

	for _, event := range <-received {
		if event.eventType != evdevEventKey || event.code != evdevKeyLeftMeta {
			continue
		}

		switch event.value {
		case evdevValuePress:
			presses++
		case evdevValueRelease:
			releases++
		}
	}

	if presses != 1 {
		t.Fatalf("Super pressed %d times before its keyboard went away, want 1", presses)
	}

	if releases != 1 {
		t.Errorf("Super released %d times after its keyboard was destroyed, want 1", releases)
	}
}
