//go:build integration && linux && cgo

package linux

// The probe against a real kernel: a uinput keyboard of the test's own is
// found under /sys/devices/virtual/input the way the proxy finds its own, and
// a grab from a second fd, which is what a remapper's auto-detect does to the
// proxy, is what the probe reads. Desktop-safe: no physical keyboard is
// touched, only the test's device.

import (
	"os"
	"testing"
	"time"
)

// proxyNodeAppearance is how long udev is given to create the event node.
const proxyNodeAppearance = 2 * time.Second

func TestProxyNode_HeldByAnother_SeesAGrabFromAnotherFd(t *testing.T) {
	keyboard, err := createTestProxyKeyboard()
	if err != nil {
		t.Skipf("/dev/uinput is not writable here, so the proxy cannot be created: %v", err)
	}

	defer keyboard.destroy()

	var node *proxyNode

	deadline := time.Now().Add(proxyNodeAppearance)
	for node == nil && time.Now().Before(deadline) {
		node = keyboard.node()
		if node == nil {
			time.Sleep(waylandEvdevIdlePollInterval)
		}
	}

	if node == nil {
		t.Fatal("the keyboard's event node was not found under /sys/devices/virtual/input")
	}

	defer func() { _ = node.file.Close() }()

	if node.heldByAnother() {
		t.Fatal("a node nobody holds reads as held by another process")
	}

	other, err := os.Open(node.file.Name())
	if err != nil {
		t.Fatalf("a second fd on the node could not be opened: %v", err)
	}

	defer func() { _ = other.Close() }()

	err = grabFile(other, true)
	if err != nil {
		t.Fatalf("the node refused a grab nobody else holds: %v", err)
	}

	if !node.heldByAnother() {
		t.Error("a grab from another fd was not seen by the probe")
	}

	err = grabFile(other, false)
	if err != nil {
		t.Fatalf("the grab could not be released: %v", err)
	}

	if node.heldByAnother() {
		t.Error("a released grab still reads as held")
	}
}
