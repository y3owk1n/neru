package axobserver

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"go.uber.org/goleak"

	"github.com/y3owk1n/neru/internal/core/ports"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// fakePlatform records observer operations and lets a test drive the sink.
type fakePlatform struct {
	mu          sync.Mutex
	armed       map[int]uint32 // pid -> currently-armed mask
	handlePID   map[unsafe.Pointer]int
	armCalls    int
	disarmCalls int
	disarmLive  []bool
	startCount  int
	stopCount   int
	running     bool
	failArm     map[int]bool
	armGate     chan struct{} // if non-nil, arm blocks until it is closed/sent
	sink        func(pid int, epoch uint64, notif string)
}

func newFakePlatform() *fakePlatform {
	return &fakePlatform{
		armed:     map[int]uint32{},
		handlePID: map[unsafe.Pointer]int{},
		failArm:   map[int]bool{},
	}
}

func (f *fakePlatform) startThread() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.running = true
	f.startCount++
}

func (f *fakePlatform) stopThread() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.running = false
	f.stopCount++
}

func (f *fakePlatform) threadRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.running
}

func (f *fakePlatform) setMessagingTimeout(float64) {}

func (f *fakePlatform) arm(pid int, _ uint64, mask uint32) unsafe.Pointer {
	f.mu.Lock()
	f.armCalls++
	gate := f.armGate
	fail := f.failArm[pid]
	f.mu.Unlock()

	// Block outside the lock so a test can stall the actor mid-arm.
	if gate != nil {
		<-gate
	}

	if fail {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// A real Go allocation gives a unique, GC-safe handle without uintptr tricks.
	handle := unsafe.Pointer(new(byte))
	f.handlePID[handle] = pid
	f.armed[pid] = mask

	return handle
}

func (f *fakePlatform) disarm(handle unsafe.Pointer, live bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.disarmCalls++
	f.disarmLive = append(f.disarmLive, live)

	if pid, ok := f.handlePID[handle]; ok {
		delete(f.armed, pid)
		delete(f.handlePID, handle)
	}
}

func (f *fakePlatform) setSink(sink func(pid int, epoch uint64, notif string)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sink = sink
}

func (f *fakePlatform) maskFor(pid int) (uint32, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, ok := f.armed[pid]

	return m, ok
}

func (f *fakePlatform) armedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.armed)
}

func (f *fakePlatform) counts() (arm, disarm, start, stop int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.armCalls, f.disarmCalls, f.startCount, f.stopCount
}

func (f *fakePlatform) callSink(pid int, epoch uint64) {
	f.mu.Lock()
	sink := f.sink
	f.mu.Unlock()

	if sink != nil {
		sink(pid, epoch, "AXTest")
	}
}

func waitFor(t *testing.T, msg string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatalf("condition not met within timeout: %s", msg)
}

func target(pid int, bundle string, src ports.ObservationSource) ports.ObservationTarget {
	return ports.ObservationTarget{PID: pid, BundleID: bundle, Source: src}
}

func TestReconcileArmsAndDisarms(t *testing.T) {
	fake := newFakePlatform()
	m := newManager(nil, Config{}, fake)
	defer m.Close()

	m.Reconcile([]ports.ObservationTarget{
		target(101, "com.example.a", ports.ObservationFrontWindow),
	})
	waitFor(t, "pid 101 armed", func() bool { _, ok := fake.maskFor(101); return ok })

	if !fake.threadRunning() {
		t.Fatal("observer thread should be running while an observer is armed")
	}

	m.Reconcile(nil)
	waitFor(t, "all disarmed", func() bool { return fake.armedCount() == 0 })

	if fake.threadRunning() {
		t.Fatal("observer thread should stop when no observers remain")
	}
}

func TestReconcileMergesMasksForSamePID(t *testing.T) {
	fake := newFakePlatform()
	m := newManager(nil, Config{}, fake)
	defer m.Close()

	m.Reconcile([]ports.ObservationTarget{
		target(7, "com.example.front", ports.ObservationFrontWindow),
		target(7, "com.example.front", ports.ObservationAppMenubar),
	})
	waitFor(t, "pid 7 armed", func() bool { _, ok := fake.maskFor(7); return ok })

	mask, _ := fake.maskFor(7)
	want := maskForSource(ports.ObservationFrontWindow, false) |
		maskForSource(ports.ObservationAppMenubar, false)

	if mask != want {
		t.Fatalf("merged mask = %#x, want %#x", mask, want)
	}

	if fake.armedCount() != 1 {
		t.Fatalf("same pid under two sources must arm exactly one observer, got %d", fake.armedCount())
	}
}

func TestSelfExclusion(t *testing.T) {
	fake := newFakePlatform()
	m := newManager(nil, Config{SelfPID: 42, SelfBundleID: "com.y3owk1n.neru"}, fake)
	defer m.Close()

	m.Reconcile([]ports.ObservationTarget{
		target(42, "com.other", ports.ObservationFrontWindow),         // excluded by pid
		target(99, "com.y3owk1n.neru", ports.ObservationFrontWindow),  // excluded by bundle
		target(100, "com.example.ok", ports.ObservationFrontWindow),   // allowed
	})
	waitFor(t, "pid 100 armed", func() bool { _, ok := fake.maskFor(100); return ok })

	if _, ok := fake.maskFor(42); ok {
		t.Fatal("own pid must not be observed")
	}

	if _, ok := fake.maskFor(99); ok {
		t.Fatal("own bundle id must not be observed")
	}

	if fake.armedCount() != 1 {
		t.Fatalf("only the allowed pid should be armed, got %d", fake.armedCount())
	}
}

func TestRecycledPIDReArms(t *testing.T) {
	fake := newFakePlatform()
	m := newManager(nil, Config{}, fake)
	defer m.Close()

	m.Reconcile([]ports.ObservationTarget{target(5, "com.app.old", ports.ObservationDock)})
	waitFor(t, "pid 5 armed (old)", func() bool { _, ok := fake.maskFor(5); return ok })

	// Same pid, different bundle id: a recycled pid must be disarmed and re-armed.
	m.Reconcile([]ports.ObservationTarget{target(5, "com.app.new", ports.ObservationDock)})

	waitFor(t, "pid 5 re-armed", func() bool {
		arm, disarm, _, _ := fake.counts()

		return arm == 2 && disarm == 1
	})
}

func TestUnchangedReconcileIsNoOp(t *testing.T) {
	fake := newFakePlatform()
	m := newManager(nil, Config{}, fake)
	defer m.Close()

	tgt := []ports.ObservationTarget{target(11, "com.app", ports.ObservationFrontWindow)}
	m.Reconcile(tgt)
	waitFor(t, "pid 11 armed", func() bool { _, ok := fake.maskFor(11); return ok })

	m.Reconcile(tgt)
	m.Reconcile(tgt)

	// Give the actor time to process the repeats, then assert no re-arm churn.
	time.Sleep(20 * time.Millisecond)

	arm, disarm, _, _ := fake.counts()
	if arm != 1 || disarm != 0 {
		t.Fatalf("unchanged reconcile churned: arm=%d disarm=%d, want arm=1 disarm=0", arm, disarm)
	}
}

func TestTerminateDisarmsDeadPIDWithoutRemoveNotification(t *testing.T) {
	fake := newFakePlatform()
	m := newManager(nil, Config{}, fake)
	defer m.Close()

	m.Reconcile([]ports.ObservationTarget{
		target(21, "com.app.alive", ports.ObservationFrontWindow),
		target(22, "com.app.dying", ports.ObservationFrontWindow),
	})
	waitFor(t, "both armed", func() bool { return fake.armedCount() == 2 })

	m.HandleAppTerminated("com.app.dying")
	waitFor(t, "dying disarmed", func() bool { _, ok := fake.maskFor(22); return !ok })

	if _, ok := fake.maskFor(21); !ok {
		t.Fatal("live app must remain observed after another app terminates")
	}

	// The terminate path passes live=false (skip RemoveNotification IPC to a dead pid).
	fake.mu.Lock()
	defer fake.mu.Unlock()

	if len(fake.disarmLive) != 1 || fake.disarmLive[0] {
		t.Fatalf("terminate disarm should be non-live, got %v", fake.disarmLive)
	}
}

func TestStaleEpochCallbackDropped(t *testing.T) {
	fake := newFakePlatform()

	var changes atomic.Int64

	m := newManager(nil, Config{OnChange: func() { changes.Add(1) }}, fake)
	defer m.Close()

	m.Reconcile([]ports.ObservationTarget{target(3, "com.app", ports.ObservationFrontWindow)})
	waitFor(t, "pid 3 armed", func() bool { _, ok := fake.maskFor(3); return ok })

	epochOld := m.curEpoch.Load()

	// A live-session callback triggers a change.
	fake.callSink(3, epochOld)
	waitFor(t, "change registered", func() bool { return changes.Load() == 1 })

	// DisarmAll bumps the epoch; a callback tagged with the old epoch is stale.
	m.DisarmAll()
	waitFor(t, "epoch bumped", func() bool { return m.curEpoch.Load() != epochOld })

	fake.callSink(3, epochOld)
	time.Sleep(20 * time.Millisecond)

	if got := changes.Load(); got != 1 {
		t.Fatalf("stale-epoch callback must be dropped: changes=%d, want 1", got)
	}
}

func TestArmFailureDoesNotLeakThread(t *testing.T) {
	fake := newFakePlatform()
	fake.failArm[404] = true

	m := newManager(nil, Config{}, fake)
	defer m.Close()

	m.Reconcile([]ports.ObservationTarget{target(404, "com.app.fails", ports.ObservationFrontWindow)})

	// The arm fails, so no observer is held and the thread must not stay running.
	waitFor(t, "thread stopped after failed arm", func() bool {
		return !fake.threadRunning() && fake.armedCount() == 0
	})
}

func TestIdempotentClose(t *testing.T) {
	fake := newFakePlatform()
	m := newManager(nil, Config{}, fake)

	m.Reconcile([]ports.ObservationTarget{target(1, "com.app", ports.ObservationFrontWindow)})
	waitFor(t, "armed", func() bool { _, ok := fake.maskFor(1); return ok })

	m.Close()
	m.Close() // must not panic or hang

	if fake.threadRunning() {
		t.Fatal("thread must be stopped after Close")
	}

	if fake.armedCount() != 0 {
		t.Fatal("all observers must be disarmed after Close")
	}
}

// Regression: a reconcile queued (from the exiting session) while the actor is
// stuck arming must not re-arm observers after DisarmAll tears them down.
func TestDisarmAllVoidsReconcileQueuedBehindIt(t *testing.T) {
	fake := newFakePlatform()
	fake.armGate = make(chan struct{})

	m := newManager(nil, Config{}, fake)
	defer m.Close()

	// The actor starts arming pid 1 and blocks on the gate.
	m.Reconcile([]ports.ObservationTarget{target(1, "com.a", ports.ObservationFrontWindow)})
	waitFor(t, "arm in progress", func() bool {
		arm, _, _, _ := fake.counts()

		return arm >= 1
	})

	// While the actor is stuck mid-arm, another refresh queues a reconcile (same
	// generation), then the user exits hints (DisarmAll bumps the generation).
	m.Reconcile([]ports.ObservationTarget{target(2, "com.b", ports.ObservationFrontWindow)})
	m.DisarmAll()

	// Let the stuck arm finish.
	close(fake.armGate)

	// The reconcile queued behind DisarmAll must not re-arm anything, and the
	// thread must be idle: hints mode has exited.
	m.flush()
	waitFor(t, "disarmed and idle after DisarmAll", func() bool {
		return fake.armedCount() == 0 && !fake.threadRunning() && m.LiveObservers() == 0
	})
}
