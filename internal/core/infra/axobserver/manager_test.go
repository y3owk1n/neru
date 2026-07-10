package axobserver

import (
	"sync"
	"testing"

	"go.uber.org/zap"
)

// Arbitrary distinct masks for the reconcile tests. Only their distinctness and
// non-zero value matter here, not which notifications they name; maskBrowser is
// a strict superset of maskFront so the re-arm-on-change test exercises a real
// mask difference.
var (
	maskFront = notifCreated | notifUIDestroyed | notifLayoutChanged |
		notifWindowCreated | notifWindowMoved | notifWindowResized
	maskBrowser = maskFront | notifLoadComplete
	maskMenu    = notifMenuOpened | notifMenuClosed
	maskAux     = notifCreated | notifUIDestroyed | notifWindowCreated
)

// fakePlatform records what the Manager asks of the platform and lets a test
// drive the change callback, so the Manager's reconcile and lifecycle logic can
// be exercised without a live accessibility tree.
type fakePlatform struct {
	mu             sync.Mutex
	armed          map[int]Mask
	armCalls       []int
	disarmedPIDs   []int
	failArm        map[int]bool
	disarmAllCalls int
	closeCalls     int
	sink           func(int, string)
}

func newFakePlatform() *fakePlatform {
	return &fakePlatform{
		armed:   make(map[int]Mask),
		failArm: make(map[int]bool),
	}
}

func (f *fakePlatform) Arm(pid int, mask Mask) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.armCalls = append(f.armCalls, pid)

	if f.failArm[pid] {
		return false
	}

	f.armed[pid] = mask

	return true
}

func (f *fakePlatform) Disarm(pid int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.armed, pid)
	f.disarmedPIDs = append(f.disarmedPIDs, pid)
}

func (f *fakePlatform) DisarmAll() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.disarmAllCalls++
	f.armed = make(map[int]Mask)
}

func (f *fakePlatform) SetSink(sink func(int, string)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sink = sink
}

func (f *fakePlatform) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closeCalls++
	f.sink = nil
	f.armed = make(map[int]Mask)
}

func (f *fakePlatform) fire(pid int) {
	f.mu.Lock()
	sink := f.sink
	f.mu.Unlock()

	if sink != nil {
		sink(pid, "AXCreated")
	}
}

func (f *fakePlatform) armedSnapshot() map[int]Mask {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make(map[int]Mask, len(f.armed))
	for pid, mask := range f.armed {
		out[pid] = mask
	}

	return out
}

func (f *fakePlatform) armCallCount(pid int) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, called := range f.armCalls {
		if called == pid {
			count++
		}
	}

	return count
}

func TestReconcileArmsAndDisarms(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Reconcile([]Target{{PID: 10, Mask: maskFront}, {PID: 20, Mask: maskMenu}})

	armed := fake.armedSnapshot()
	if len(armed) != 2 || armed[10] != maskFront || armed[20] != maskMenu {
		t.Fatalf("after first reconcile armed = %v", armed)
	}

	manager.Reconcile([]Target{{PID: 20, Mask: maskMenu}, {PID: 30, Mask: maskAux}})

	armed = fake.armedSnapshot()
	if _, ok := armed[10]; ok {
		t.Error("pid 10 should have been disarmed")
	}

	if armed[20] != maskMenu {
		t.Error("pid 20 should have stayed armed")
	}

	if armed[30] != maskAux {
		t.Error("pid 30 should have been armed")
	}
}

func TestReconcileKeepsUnchangedTarget(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Reconcile([]Target{{PID: 10, Mask: maskFront}})
	manager.Reconcile([]Target{{PID: 10, Mask: maskFront}})

	if got := fake.armCallCount(10); got != 1 {
		t.Fatalf("unchanged target should not re-arm; arm calls = %d", got)
	}

	if len(fake.disarmedPIDs) != 0 {
		t.Fatalf("unchanged target should not disarm; disarmed = %v", fake.disarmedPIDs)
	}
}

func TestReconcileReArmsOnMaskChange(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Reconcile([]Target{{PID: 10, Mask: maskFront}})
	manager.Reconcile([]Target{{PID: 10, Mask: maskBrowser}})

	if got := fake.armCallCount(10); got != 2 {
		t.Fatalf("mask change should re-arm pid 10; arm calls = %d", got)
	}

	if got := fake.armedSnapshot()[10]; got != maskBrowser {
		t.Fatalf("armed mask = %v, want maskBrowser", got)
	}
}

func TestReconcileSkipsInvalidTargets(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Reconcile([]Target{
		{PID: 0, Mask: maskFront},
		{PID: 10, Mask: 0},
		{PID: -1, Mask: maskMenu},
	})

	if got := fake.armedSnapshot(); len(got) != 0 {
		t.Fatalf("no invalid target should arm; armed = %v", got)
	}
}

func TestReconcileArmFailureRetries(t *testing.T) {
	fake := newFakePlatform()
	fake.failArm[10] = true
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Reconcile([]Target{{PID: 10, Mask: maskFront}})
	if len(fake.armedSnapshot()) != 0 {
		t.Fatal("a failed arm should not be tracked as armed")
	}

	fake.failArm[10] = false
	manager.Reconcile([]Target{{PID: 10, Mask: maskFront}})

	if got := fake.armCallCount(10); got != 2 {
		t.Fatalf("a previously failed pid should be retried; arm calls = %d", got)
	}

	if fake.armedSnapshot()[10] != maskFront {
		t.Fatal("the retry should have armed pid 10")
	}
}

func TestDisarmAllIsNoOpWhenEmpty(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Reconcile([]Target{{PID: 10, Mask: maskFront}, {PID: 20, Mask: maskMenu}})
	manager.DisarmAll()

	if fake.disarmAllCalls != 1 {
		t.Fatalf("DisarmAll platform calls = %d, want 1", fake.disarmAllCalls)
	}

	manager.DisarmAll()

	if fake.disarmAllCalls != 1 {
		t.Fatalf("DisarmAll with nothing armed should not hit the platform; calls = %d", fake.disarmAllCalls)
	}
}

func TestChangeCallbackDeliversPID(t *testing.T) {
	fake := newFakePlatform()

	var got int
	newWithPlatform(fake, func(pid int) { got = pid }, zap.NewNop())

	fake.fire(42)

	if got != 42 {
		t.Fatalf("onChange pid = %d, want 42", got)
	}
}

func TestCloseStopsReconcileAndCallbacks(t *testing.T) {
	fake := newFakePlatform()

	fired := 0
	manager := newWithPlatform(fake, func(int) { fired++ }, zap.NewNop())

	manager.Reconcile([]Target{{PID: 10, Mask: maskFront}})
	manager.Close()

	if fake.closeCalls != 1 {
		t.Fatalf("Close platform calls = %d, want 1", fake.closeCalls)
	}

	manager.Reconcile([]Target{{PID: 20, Mask: maskMenu}})
	if len(fake.armedSnapshot()) != 0 {
		t.Fatal("Reconcile after Close should arm nothing")
	}

	fake.fire(10)
	if fired != 0 {
		t.Fatalf("no callback should fire after Close; fired = %d", fired)
	}

	manager.Close()
	if fake.closeCalls != 1 {
		t.Fatalf("a second Close should be a no-op; calls = %d", fake.closeCalls)
	}
}
