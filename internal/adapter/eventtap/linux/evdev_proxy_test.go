//go:build linux && cgo

package linux

// The proxy is driven here without devices, a uinput keyboard or its run
// goroutine: events are handed to handle directly, so what is pinned is the
// routing — which presses reach the compositor, the hotkey matcher and the mode,
// and the one invariant the instant grab rests on: a release goes wherever its
// press went.

import (
	"errors"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

// testChord is the binding these tests press, spelled as a user would.
const testChord = "Super+;"

func keyEvent(code uint16, value int32) waylandEvdevEvent {
	return waylandEvdevEvent{eventType: evdevEventKey, code: code, value: value}
}

// newTestProxy is a passive proxy with no devices: emit is a no-op, so the
// forward rule's bits are the record of what would have reached the compositor.
func newTestProxy() *evdevProxy {
	proxy := &evdevProxy{
		logger:   zap.NewNop(),
		capture:  &waylandEvdevCapture{logger: zap.NewNop()},
		uinputFd: -1,
		global:   newWaylandEvdevKeyState(),
		control:  make(chan proxyCommand),
		done:     make(chan struct{}),
	}

	empty := map[string]hotkeyBinding{}
	proxy.bindings.Store(&empty)
	proxy.heldHotkeys = make(map[uint16]func())

	return proxy
}

func bindTestChord(proxy *evdevProxy) chan struct{} {
	fired := make(chan struct{}, 4)

	proxy.setBindings(map[string]hotkeyBinding{
		canonicalChordSignature(testChord): {press: func() { fired <- struct{}{} }},
	})

	return fired
}

// waitFired reports whether the binding ran within a generous window. The
// callback is dispatched on a goroutine, so a miss has to be waited for.
func waitFired(t *testing.T, fired chan struct{}) bool {
	t.Helper()

	select {
	case <-fired:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}

// collectKeys is a tap whose dispatched keys land on the returned channel.
func collectKeys(t *testing.T) (*EventTap, chan string) {
	t.Helper()

	keys := make(chan string, 16)
	tap := NewEventTap(func(key string) { keys <- key }, nil)
	t.Cleanup(tap.Destroy)

	return tap, keys
}

func nextKey(t *testing.T, keys chan string) string {
	t.Helper()

	select {
	case key := <-keys:
		return key
	case <-time.After(2 * time.Second):
		t.Fatal("no key was dispatched")

		return ""
	}
}

func noKey(t *testing.T, keys chan string) {
	t.Helper()

	select {
	case key := <-keys:
		t.Fatalf("dispatched %q, want nothing", key)
	case <-time.After(50 * time.Millisecond):
	}
}

// beginSession puts a session in force the way the run goroutine would.
func beginSession(proxy *evdevProxy, tap *EventTap) *evdevSession {
	session := newEvdevSession(tap)
	proxy.session = session
	session.begin(proxy)

	return session
}

func TestForwardRule_AReleaseGoesWhereItsPressWent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		withholdPress bool
		seeded        bool
		wantPress     bool
		wantRepeat    bool
		wantRelease   bool
	}{
		{name: "idle: forwarded throughout", wantPress: true, wantRepeat: true, wantRelease: true},
		{name: "capturing: withheld throughout", withholdPress: true},
		{
			name:   "re-emitted by the proxy: the compositor saw the press, so it gets the release",
			seeded: true, withholdPress: true, wantRepeat: true, wantRelease: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var rule forwardRule

			if testCase.seeded {
				rule.seed(evdevKeyA)
			} else if got := rule.press(evdevKeyA, testCase.withholdPress); got != testCase.wantPress {
				t.Errorf("press forwarded = %v, want %v", got, testCase.wantPress)
			}

			if got := rule.repeat(evdevKeyA); got != testCase.wantRepeat {
				t.Errorf("repeat forwarded = %v, want %v", got, testCase.wantRepeat)
			}

			if got := rule.release(evdevKeyA); got != testCase.wantRelease {
				t.Errorf("release forwarded = %v, want %v", got, testCase.wantRelease)
			}

			if rule.isDown(evdevKeyA) {
				t.Error("the key is still down after its release")
			}
		})
	}
}

// One key held on two keyboards is one key to the compositor: it goes up when
// the last of them lets go, not the first.
func TestForwardRule_AKeyHeldOnTwoKeyboardsReleasesWithTheLast(t *testing.T) {
	t.Parallel()

	var rule forwardRule

	rule.press(evdevKeyA, false)
	rule.press(evdevKeyA, true) // the second keyboard, during a mode

	if rule.release(evdevKeyA) {
		t.Error("the first release was forwarded while the other keyboard still holds the key")
	}

	if !rule.isDown(evdevKeyA) {
		t.Error("the key is up after one of two holds ended")
	}

	if !rule.release(evdevKeyA) {
		t.Error("the last release was withheld")
	}
}

func TestForwardRule_IgnoresCodesOutsideTheKeyRange(t *testing.T) {
	t.Parallel()

	var rule forwardRule

	rule.press(evdevKeyCodeCount, false)

	if rule.isDown(evdevKeyCodeCount) {
		t.Fatal("a code past KEY_MAX was recorded")
	}
}

// A held chord is one press and one release to the binder, which is what lets
// it repeat the binding for as long as the key is down: the kernel's repeats
// fire nothing, and the release comes when the key comes up, whether or not
// the modifier is still held by then.
func TestEvdevProxy_AHeldChordFiresPressOnceAndReleaseWhenItsKeyComesUp(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	pressed := make(chan struct{}, 4)
	released := make(chan struct{}, 4)

	proxy.setBindings(map[string]hotkeyBinding{
		canonicalChordSignature(testChord): {
			press:   func() { pressed <- struct{}{} },
			release: func() { released <- struct{}{} },
		},
	})

	proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValuePress))
	proxy.handle(keyEvent(evdevKeySemicolon, evdevValuePress))

	if !waitFired(t, pressed) {
		t.Fatal("the chord did not match")
	}

	proxy.handle(keyEvent(evdevKeySemicolon, evdevValueRepeat))
	proxy.handle(keyEvent(evdevKeySemicolon, evdevValueRepeat))

	select {
	case <-pressed:
		t.Fatal(
			"a repeat fired the press again; the binder would restart its repeat from the delay",
		)
	case <-released:
		t.Fatal("the release fired while the key was still down")
	case <-time.After(50 * time.Millisecond):
	}

	// The modifier comes up first, so the key's release is no longer the
	// chord. The release is owed to the key, not the chord.
	proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValueRelease))
	proxy.handle(keyEvent(evdevKeySemicolon, evdevValueRelease))

	if !waitFired(t, released) {
		t.Fatal("the key came up and no release fired; the binder would repeat forever")
	}
}

// A matched chord is withheld from the focused app, the way the macOS tap
// consumes it, while the modifier that was already down when it matched stays
// the compositor's and is released to it.
func TestEvdevProxy_MatchesAChordAndWithholdsIt(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	fired := bindTestChord(proxy)

	proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValuePress))
	proxy.handle(keyEvent(evdevKeySemicolon, evdevValuePress))

	if !waitFired(t, fired) {
		t.Fatal("the chord did not match")
	}

	if !proxy.rule.isDown(evdevKeyLeftMeta) {
		t.Error(
			"the chord's modifier was withheld; the compositor saw its press and is owed its release",
		)
	}

	if proxy.rule.isDown(evdevKeySemicolon) {
		t.Error("the chord's key was forwarded; the focused app received the activation chord")
	}

	if forwarded := proxy.rule.release(evdevKeySemicolon); forwarded {
		t.Error("the key's release was forwarded after its press was withheld")
	}

	if forwarded := proxy.rule.release(evdevKeyLeftMeta); !forwarded {
		t.Error("the modifier's release was withheld after its press was forwarded")
	}
}

// The ordinary case, so the guards above cannot pass by matching nothing: a
// chord pressed and released leaves nothing behind and matches every time.
func TestEvdevProxy_MatchesTheChordEveryTime(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	fired := bindTestChord(proxy)

	for attempt := 1; attempt <= 3; attempt++ {
		proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValuePress))
		proxy.handle(keyEvent(evdevKeySemicolon, evdevValuePress))
		proxy.handle(keyEvent(evdevKeySemicolon, evdevValueRelease))
		proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValueRelease))

		if !waitFired(t, fired) {
			t.Fatalf("the chord did not match on attempt %d", attempt)
		}

		if proxy.global.modifiers.cmd != 0 {
			t.Fatalf(
				"cmd count is %d after attempt %d, want 0",
				proxy.global.modifiers.cmd,
				attempt,
			)
		}
	}
}

// A mode that starts under its activation chord neither waits for the chord to
// come up nor reads it: the label typed while Super is still down is the label.
// The chord's modifier release goes to the compositor and is not a sticky
// toggle, and sticky detection arms once the chord has come up.
func TestEvdevProxy_SessionStartsUnderTheChordWithoutCountingIt(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	tap, keys := collectKeys(t)
	tap.SetStickyModifierToggle(true)

	// The chord matched and the mode is opening while Super is still down.
	proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValuePress))
	session := beginSession(proxy, tap)

	if tap.stickyDetectionArmed() {
		t.Fatal("sticky detection armed while the activation chord's modifier is still down")
	}

	proxy.handle(keyEvent(evdevKeyA, evdevValuePress))

	if got := nextKey(t, keys); got != "a" {
		t.Fatalf("dispatched %q under the held activation modifier, want %q", got, "a")
	}

	if proxy.rule.isDown(evdevKeyA) {
		t.Error("a press during the session was forwarded to the compositor")
	}

	proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValueRelease))
	noKey(t, keys)

	if len(session.forwardedModifiers) != 0 {
		t.Error("the chord's modifier is still counted after its release")
	}

	if !tap.stickyDetectionArmed() {
		t.Error("sticky detection did not arm once the activation chord came up")
	}

	// From here a modifier tap is the user's, and a sticky toggle.
	proxy.handle(keyEvent(evdevKeyLeftShift, evdevValuePress))
	proxy.handle(keyEvent(evdevKeyLeftShift, evdevValueRelease))

	if got := nextKey(t, keys); got != "__modifier_shift_down" {
		t.Fatalf("dispatched %q for a shift press, want a sticky toggle", got)
	}

	if got := nextKey(t, keys); got != "__modifier_shift_up" {
		t.Fatalf("dispatched %q for a shift release, want a sticky toggle", got)
	}
}

// A press withheld for the mode is followed by its release: the mode sees both,
// the compositor neither. A release for a press the mode never saw reaches it
// no more than the press did.
func TestEvdevProxy_SessionSeesAReleaseOnlyForAPressItSaw(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	tap, keys := collectKeys(t)

	proxy.handle(keyEvent(evdevKeyA, evdevValuePress))
	beginSession(proxy, tap)

	// Held before the session: forwarded, and the release follows it out.
	proxy.handle(keyEvent(evdevKeyA, evdevValueRepeat))
	proxy.handle(keyEvent(evdevKeyA, evdevValueRelease))
	noKey(t, keys)

	proxy.handle(keyEvent(evdevKeyB, evdevValuePress))

	if got := nextKey(t, keys); got != "b" {
		t.Fatalf("dispatched %q, want %q", got, "b")
	}

	proxy.handle(keyEvent(evdevKeyB, evdevValueRepeat))

	if got := nextKey(t, keys); got != "b" {
		t.Fatalf("dispatched %q for a repeat, want %q", got, "b")
	}

	proxy.handle(keyEvent(evdevKeyB, evdevValueRelease))

	if got := nextKey(t, keys); got != "__keyup_b" {
		t.Fatalf("dispatched %q for the release, want a key-up", got)
	}

	if proxy.rule.isDown(evdevKeyB) {
		t.Error("a withheld key is recorded as forwarded")
	}
}

// A repeat with no press behind it is the kernel repeating a key the session
// never saw pressed, and reaches nothing.
func TestEvdevProxy_SessionIgnoresARepeatWithoutAPress(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	tap, keys := collectKeys(t)

	beginSession(proxy, tap)

	proxy.handle(keyEvent(evdevKeyU, evdevValueRepeat))
	noKey(t, keys)
}

// An unbound modifier chord the mode lets through goes out on the proxy
// keyboard with the modifiers the user is holding, and every one of them stays
// the compositor's until it is physically released: no re-tap per repeat and
// nothing to release when the mode ends.
func TestEvdevProxy_PassthroughForwardsTheChordAndItsHeldModifiers(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	tap, keys := collectKeys(t)
	tap.SetModifierPassthrough(true, nil)
	tap.SetPassthroughCallback(func() { keys <- "passthrough" })

	beginSession(proxy, tap)

	proxy.handle(keyEvent(evdevKeyLeftCtrl, evdevValuePress))

	if proxy.rule.isDown(evdevKeyLeftCtrl) {
		t.Fatal("a modifier pressed during the session was forwarded before anything used it")
	}

	proxy.handle(keyEvent(evdevKeyC, evdevValuePress))

	if got := nextKey(t, keys); got != "passthrough" {
		t.Fatalf("got %q, want the passthrough callback and no dispatch", got)
	}

	if !proxy.rule.isDown(evdevKeyLeftCtrl) || !proxy.rule.isDown(evdevKeyC) {
		t.Fatal("the passed-through chord is not recorded as forwarded")
	}

	// Repeats and releases follow the press out, and none of it reaches the mode.
	proxy.handle(keyEvent(evdevKeyC, evdevValueRepeat))
	proxy.handle(keyEvent(evdevKeyC, evdevValueRelease))
	proxy.handle(keyEvent(evdevKeyLeftCtrl, evdevValueRelease))
	noKey(t, keys)

	if proxy.rule.isDown(evdevKeyLeftCtrl) || proxy.rule.isDown(evdevKeyC) {
		t.Error("the passed-through chord is still down after its releases")
	}
}

// A proxy whose device stopped taking writes lets go of the keyboards rather
// than hold keys it can no longer deliver, and refuses to capture for a mode
// from then on, so the tap falls back to the overlay's keyboard focus.
// A remapper's output keyboard also carries mouse motion and buttons. Those are
// not keys: a mode does not capture a click, the matcher does not spell a chord
// with one, and the forward rule owes the compositor nothing for it.
func TestEvdevProxy_PointerEventsAreNotKeys(t *testing.T) {
	t.Parallel()

	const btnLeft uint16 = 0x110

	proxy := newTestProxy()
	fired := bindTestChord(proxy)
	tap, keys := collectKeys(t)
	beginSession(proxy, tap)

	proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValuePress))
	proxy.handle(keyEvent(btnLeft, evdevValuePress))
	proxy.handle(waylandEvdevEvent{eventType: evdevEventRel, code: 0, value: 3})
	proxy.handle(waylandEvdevEvent{eventType: evdevEventSyn})
	proxy.handle(keyEvent(btnLeft, evdevValueRelease))
	proxy.handle(waylandEvdevEvent{eventType: evdevEventSyn})

	if proxy.rule.isDown(btnLeft) {
		t.Error("a mouse button was counted as a forwarded key")
	}

	if proxy.global.pressed[btnLeft] {
		t.Error("a mouse button was tracked as a held key")
	}

	if waitFired(t, fired) {
		t.Error("a binding fired on pointer events")
	}

	noKey(t, keys)

	if proxy.pointerFrame {
		t.Error("the pointer frame was left open after its sync report")
	}
}

func TestEvdevProxy_FailOpenReleasesTheKeyboardsAndRefusesSessions(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	proxy.forwarding.Store(true)
	proxy.capture.grab = true

	tap, keys := collectKeys(t)
	session := beginSession(proxy, tap)

	proxy.failOpen(errWaylandEvdevProxyStopped)

	if proxy.forwarding.Load() {
		t.Error("the proxy still reports forwarding after a failed write")
	}

	if proxy.capture.grab {
		t.Error("the capture would still grab a keyboard that arrives later")
	}

	select {
	case <-session.ended:
	default:
		t.Error(
			"the mode session was left attached to a proxy whose keyboards now reach the app too",
		)
	}

	proxy.handle(keyEvent(evdevKeyA, evdevValuePress))
	noKey(t, keys)

	err := proxy.startSession(newEvdevSession(nil))
	if err == nil {
		t.Error("a session was accepted with nothing to re-emit keys through")
	}
}

// A key down when the proxy fails open had its press re-emitted on the proxy
// keyboard, and its release goes to the physical device next, where libinput
// never saw the press: the proxy releases what it forwarded before it lets
// the keyboards go, or the key stays down on the proxy for the daemon's life.
func TestEvdevProxy_FailOpen_ReleasesEveryKeyItForwarded(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()

	// The activation chord, forwarded: no session is capturing.
	proxy.handle(keyEvent(evdevKeyLeftMeta, evdevValuePress))
	proxy.handle(keyEvent(evdevKeyJ, evdevValuePress))

	if !proxy.rule.isDown(evdevKeyLeftMeta) || !proxy.rule.isDown(evdevKeyJ) {
		t.Fatal("the chord was not forwarded before the proxy failed open")
	}

	proxy.forwarding.Store(true)
	proxy.capture.grab = true
	proxy.failOpen(errWaylandEvdevProxyGrabbed)

	for _, code := range []uint16{evdevKeyLeftMeta, evdevKeyJ} {
		if proxy.rule.isDown(code) {
			t.Errorf("key %d is still down on the proxy keyboard after it failed open", code)
		}
	}

	// The physical releases that follow are the physical device's now.
	if proxy.rule.release(evdevKeyJ) || proxy.rule.release(evdevKeyLeftMeta) {
		t.Error("a release was forwarded for a key the fail-open already released")
	}
}

// A keyboard yielded to a remapper is with the compositor until the remapper
// claims it or it is taken back; a mode started meanwhile would get nothing
// while its keys went to the focused app, so it is refused and falls back.
func TestEvdevProxy_StartSession_RefusesWhileAKeyboardIsYielded(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	proxy.forwarding.Store(true)
	proxy.capture.grab = true
	proxy.capture.files = []*os.File{nil}
	proxy.capture.yielded = 1

	err := proxy.startSession(newEvdevSession(nil))
	if !errors.Is(err, errWaylandEvdevYielded) {
		t.Fatalf("startSession error = %v, want the keyboards reported as yielded", err)
	}

	if proxy.capture.sessionActive {
		t.Error("a refused session was recorded as capturing keys")
	}
}

// A remapper's device auto-detect grabbing a proxy device closes a loop the
// compositor sees no side of; the probe on the run goroutine is what opens it.
func TestEvdevProxy_ProbeOwnDevices_FailsOpenWhenAnotherProcessHoldsAProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		held func(proxy *evdevProxy) *proxyNode
	}{
		{
			name: "the keyboard proxy is held",
			held: func(proxy *evdevProxy) *proxyNode { return proxy.keyboardNode },
		},
		{
			name: "the pointer proxy is held",
			held: func(proxy *evdevProxy) *proxyNode { return proxy.pointerNode },
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			proxy := newTestProxy()
			proxy.forwarding.Store(true)
			proxy.capture.grab = true
			proxy.keyboardNode = &proxyNode{}
			proxy.pointerNode = &proxyNode{}

			taken := false
			probes := 0
			proxy.heldByAnother = func(node *proxyNode) bool {
				probes++

				return taken && node == testCase.held(proxy)
			}

			proxy.probeOwnDevices()

			if !proxy.forwarding.Load() {
				t.Fatal("a probe that found both nodes free failed open")
			}

			taken = true

			proxy.probeOwnDevices()

			if proxy.forwarding.Load() {
				t.Error("the proxy still forwards with another process holding its device")
			}

			if proxy.capture.grab {
				t.Error("the capture would still grab a keyboard that arrives later")
			}

			err := proxy.startSession(newEvdevSession(nil))
			if err == nil {
				t.Error("a session was accepted with the keyboards released")
			}

			before := probes

			proxy.probeOwnDevices()

			if probes != before {
				t.Error("a proxy that has let go keeps probing its devices")
			}
		})
	}
}

// Detaching the bindings is all a stop does now: the proxy keeps being the
// keyboard, and a listener that has stopped matches nothing.
func TestGlobalHotkeyListener_StopDetachesTheBindings(t *testing.T) {
	t.Parallel()

	proxy := newTestProxy()
	listener := NewGlobalHotkeyListener(nil)
	listener.SetBinding(testChord, func() {}, nil)

	listener.mu.Lock()
	listener.proxy = proxy
	listener.running = true
	listener.publishLocked()
	listener.mu.Unlock()

	if len(*proxy.bindings.Load()) != 1 {
		t.Fatal("the running listener published no bindings")
	}

	listener.Stop()

	if len(*proxy.bindings.Load()) != 0 {
		t.Error("a stopped listener left its bindings on the proxy")
	}

	if listener.IsRunning() {
		t.Error("the listener still reports itself running after Stop")
	}
}
