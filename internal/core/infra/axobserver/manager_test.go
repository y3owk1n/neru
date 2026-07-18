package axobserver

import (
	"sync"
	"testing"

	"go.uber.org/zap"
)

// fakePlatform records what the Manager asks of the platform and lets a test
// drive the change callback, so the Manager's watch and lifecycle logic can be
// exercised without a live accessibility tree.
type fakePlatform struct {
	mu             sync.Mutex
	armed          map[int]struct{}
	armCalls       []int
	disarmedPIDs   []int
	failArm        map[int]bool
	disarmAllCalls int
	closeCalls     int
	handler        func(int, string)
}

func newFakePlatform() *fakePlatform {
	return &fakePlatform{
		armed:   make(map[int]struct{}),
		failArm: make(map[int]bool),
	}
}

func (f *fakePlatform) Arm(pid int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.armCalls = append(f.armCalls, pid)

	if f.failArm[pid] {
		return false
	}

	f.armed[pid] = struct{}{}

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
	f.armed = make(map[int]struct{})
}

func (f *fakePlatform) SetChangeHandler(handler func(int, string)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.handler = handler
}

func (f *fakePlatform) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closeCalls++
	f.handler = nil
	f.armed = make(map[int]struct{})
}

func (f *fakePlatform) fire(pid int) {
	f.mu.Lock()
	handler := f.handler
	f.mu.Unlock()

	if handler != nil {
		handler(pid, "AXCreated")
	}
}

func (f *fakePlatform) armedSnapshot() map[int]struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make(map[int]struct{}, len(f.armed))
	for pid := range f.armed {
		out[pid] = struct{}{}
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

func TestWatchArmsAndSwaps(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Watch(10)

	armed := fake.armedSnapshot()
	if _, ok := armed[10]; !ok || len(armed) != 1 {
		t.Fatalf("after Watch(10) armed = %v", armed)
	}

	manager.Watch(20)

	armed = fake.armedSnapshot()
	if _, ok := armed[10]; ok {
		t.Error("pid 10 should have been disarmed after the swap")
	}

	if _, ok := armed[20]; !ok || len(armed) != 1 {
		t.Errorf("pid 20 should be the only armed pid; armed = %v", armed)
	}

	if got := fake.armCallCount(20); got != 1 {
		t.Errorf("pid 20 arm calls = %d, want 1", got)
	}
}

func TestWatchIsNoOpForSamePID(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Watch(10)
	manager.Watch(10)

	if got := fake.armCallCount(10); got != 1 {
		t.Fatalf("watching the same pid should not re-arm; arm calls = %d", got)
	}

	if len(fake.disarmedPIDs) != 0 {
		t.Fatalf("watching the same pid should not disarm; disarmed = %v", fake.disarmedPIDs)
	}
}

func TestWatchIgnoresNonPositivePID(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Watch(0)
	manager.Watch(-1)

	if got := fake.armedSnapshot(); len(got) != 0 {
		t.Fatalf("a non-positive pid should arm nothing; armed = %v", got)
	}

	if len(fake.armCalls) != 0 {
		t.Fatalf("a non-positive pid should not reach the platform; arm calls = %v", fake.armCalls)
	}
}

func TestWatchArmFailureRetries(t *testing.T) {
	fake := newFakePlatform()
	fake.failArm[10] = true
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Watch(10)

	if len(fake.armedSnapshot()) != 0 {
		t.Fatal("a failed arm should not be tracked as armed")
	}

	fake.failArm[10] = false

	manager.Watch(10)

	if got := fake.armCallCount(10); got != 2 {
		t.Fatalf("a previously failed pid should be retried; arm calls = %d", got)
	}

	if _, ok := fake.armedSnapshot()[10]; !ok {
		t.Fatal("the retry should have armed pid 10")
	}
}

func TestUnwatchDisarms(t *testing.T) {
	fake := newFakePlatform()
	manager := newWithPlatform(fake, nil, zap.NewNop())

	manager.Watch(10)
	manager.Unwatch()

	if fake.disarmAllCalls != 1 {
		t.Fatalf("Unwatch platform calls = %d, want 1", fake.disarmAllCalls)
	}

	manager.Unwatch()

	if fake.disarmAllCalls != 1 {
		t.Fatalf(
			"Unwatch with nothing armed should not hit the platform; calls = %d",
			fake.disarmAllCalls,
		)
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

func TestCloseStopsWatchAndCallbacks(t *testing.T) {
	fake := newFakePlatform()

	fired := 0
	manager := newWithPlatform(fake, func(int) { fired++ }, zap.NewNop())

	manager.Watch(10)
	manager.Close()

	if fake.closeCalls != 1 {
		t.Fatalf("Close platform calls = %d, want 1", fake.closeCalls)
	}

	manager.Watch(20)

	if len(fake.armedSnapshot()) != 0 {
		t.Fatal("Watch after Close should arm nothing")
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
