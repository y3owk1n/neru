// The evdev keyboard proxy: every physical keyboard is held (EVIOCGRAB) for the
// daemon's lifetime and re-emitted through one uinput keyboard, so the
// compositor's libinput only ever sees a device Neru controls. Capturing keys
// for a mode is then a routing decision on the run goroutine — the same shape
// as the macOS CGEventTap — rather than a grab that has to be acquired, and
// waited for, on every activation.
//
// One goroutine reads every device, applies the forward rule, and hands each
// event to whichever consumer is current: the mode session while one is open,
// the global-hotkey matcher otherwise. Exactly one of the two sees any given
// press, which is what keeps one press from running a binding twice.

//go:build linux && cgo

package linux

/*
#include "../../platform/linux/evdev.h"
#include "../../platform/linux/wayland_keymap.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

type evdevProxy struct {
	logger  *zap.Logger
	capture *waylandEvdevCapture

	// uinputFd is the proxy keyboard, or -1 when /dev/uinput could not be
	// opened. Without it the proxy is passive: devices are read alongside the
	// compositor rather than grabbed, hotkeys still match, and a mode session
	// is refused — there is nothing to re-emit the keys it would not capture.
	uinputFd C.int

	// forwarding is whether keys are re-emitted on uinputFd. It starts true
	// with the device and goes false for good if a write to it ever fails
	// (failOpen), which is the one way the proxy can lose the keyboard after
	// grabbing it — so that is when it lets go.
	forwarding atomic.Bool

	// Owned by the run goroutine.
	rule    forwardRule
	global  waylandEvdevKeyState
	session *evdevSession

	// pointerFrame is whether the frame being read has put anything on the
	// pointer proxy, so its sync report is sent there too.
	pointerFrame bool

	// keyboardNode and pointerNode are the proxy's own event nodes, opened on
	// the run goroutine once udev has created them, and probed from there for
	// a grab by another process (probeOwnDevices). heldByAnother is the probe,
	// a field so a test can stand in for the kernel.
	keyboardNode  *proxyNode
	pointerNode   *proxyNode
	heldByAnother func(*proxyNode) bool

	bindings atomic.Pointer[map[string]func()]

	control chan proxyCommand
	done    chan struct{}
}

// proxyCommand replaces the current session (nil ends it). The run goroutine
// closes ack once the replacement is in force, so the caller knows no event
// will reach the old session after it returns.
type proxyCommand struct {
	session *evdevSession
	ack     chan struct{}
}

var (
	sharedProxyMu sync.Mutex
	sharedProxy   *evdevProxy
)

// acquireEvdevProxy returns the process-wide proxy, building it on first use.
// A failed build can be retried by the next caller: what failed was reading
// /dev/input, which a login into the input group later fixes.
//
// The proxy lives for the process. Nothing closes it: the tap and the hotkey
// listener both borrow it, and the kernel drops every grab and the uinput
// device when the daemon exits, which is the recovery a crashed daemon gets too.
func acquireEvdevProxy(logger *zap.Logger) (*evdevProxy, error) {
	sharedProxyMu.Lock()
	defer sharedProxyMu.Unlock()

	if sharedProxy != nil {
		return sharedProxy, nil
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.Named("evdev")

	proxy, err := newEvdevProxy(logger)
	if err != nil {
		return nil, err
	}

	sharedProxy = proxy

	return proxy, nil
}

// WarmEvdevProxy builds the proxy at daemon start, so the keyboards are held
// before any key that could be down at a mode's activation. Built lazily by the
// first mode instead, the proxy met a user who launches modes from compositor
// bindings with the binding's modifier still down at grab time: the device then
// waits for that key to come up (waitIdle), and a session asked for inside the
// wait is refused. The hotkey listener and the tap borrow the proxy afterwards.
func WarmEvdevProxy(logger *zap.Logger) error {
	_, err := acquireEvdevProxy(logger)

	return err
}

func newEvdevProxy(logger *zap.Logger) (*evdevProxy, error) {
	var uinputFd C.int = -1

	created, errno := C.neru_uinput_create_proxy_keyboard(&uinputFd)
	if created == 0 {
		// Warn, not error: hotkeys still work. What does not is capturing
		// keys for a mode, and the tap says so again when a mode asks.
		logger.Warn(
			"Keyboard capture unavailable: /dev/uinput is not writable, so modes fall back "+
				"to the overlay's keyboard focus (add a udev rule or the input group to grant access)",
			zap.Error(errno),
		)
	}

	capture, err := newWaylandEvdevCapture(logger, uinputFd >= 0)
	if err != nil {
		if uinputFd >= 0 {
			C.neru_uinput_destroy(uinputFd)
		}

		return nil, fmt.Errorf("%w: %w", errWaylandEvdevUnavailable, err)
	}

	capture.refreshXkbState()

	proxy := &evdevProxy{
		logger:        logger,
		capture:       capture,
		uinputFd:      uinputFd,
		global:        newWaylandEvdevKeyState(),
		control:       make(chan proxyCommand),
		done:          make(chan struct{}),
		heldByAnother: (*proxyNode).heldByAnother,
	}

	proxy.forwarding.Store(uinputFd >= 0)

	empty := map[string]func(){}
	proxy.bindings.Store(&empty)

	go proxy.run()

	capture.startReaders()

	if uinputFd >= 0 {
		go proxy.ledLoop()
	}

	logger.Info(
		"Evdev keyboard proxy running",
		zap.Int("devices", capture.deviceCount()),
		zap.Bool("forwarding", proxy.forwarding.Load()),
	)

	return proxy, nil
}

func (p *evdevProxy) deviceCount() int {
	return p.capture.deviceCount()
}

func (p *evdevProxy) alive() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// setBindings replaces the chords the idle matcher fires on. The map is read
// on the run goroutine without a lock, so it is swapped whole and never edited.
func (p *evdevProxy) setBindings(bindings map[string]func()) {
	copied := make(map[string]func(), len(bindings))
	maps.Copy(copied, bindings)

	p.bindings.Store(&copied)
}

// startSession routes every key to session from the next event on, and returns
// once that is so.
func (p *evdevProxy) startSession(session *evdevSession) error {
	if !p.forwarding.Load() {
		return errWaylandEvdevPassive
	}

	if p.deviceCount() == 0 {
		if p.capture.pendingCount() > 0 {
			return errWaylandEvdevGrabPending
		}

		return errWaylandEvdevUnavailable
	}

	return p.send(proxyCommand{session: session, ack: make(chan struct{})})
}

// stopSession returns keys to the compositor and the idle matcher, and returns
// once no event can reach the session any more.
func (p *evdevProxy) stopSession() {
	_ = p.send(proxyCommand{ack: make(chan struct{})})
}

func (p *evdevProxy) send(cmd proxyCommand) error {
	select {
	case p.control <- cmd:
	case <-p.done:
		return errWaylandEvdevProxyStopped
	}

	select {
	case <-cmd.ack:
		return nil
	case <-p.done:
		return errWaylandEvdevProxyStopped
	}
}

func (p *evdevProxy) run() {
	defer close(p.done)

	keymap := time.NewTicker(waylandEvdevKeymapPollInterval)
	defer keymap.Stop()

	probe := time.NewTicker(waylandEvdevProxyProbeInterval)
	defer probe.Stop()

	for {
		select {
		case cmd := <-p.control:
			p.session = cmd.session

			if cmd.session != nil {
				cmd.session.begin(p)
				p.refreshKeymap()
			}

			close(cmd.ack)
		case event, ok := <-p.capture.events:
			if !ok {
				return
			}

			p.handle(event)
		case <-keymap.C:
			p.refreshKeymap()
		case <-probe.C:
			p.probeOwnDevices()
		}
	}
}

// probeOwnDevices fails open when another process has grabbed one of the
// proxy's own devices. That is what a key remapper's device auto-detect does
// to a new keyboard node, and with the remapper's output grabbed here it
// closes a loop that reaches the compositor from neither side: every key the
// user types circles between the two processes, and the keyboard is dead until
// one of them lets go. Letting go here returns the remapper's output to the
// compositor, so the user's keys and remaps work again; what is lost is
// capturing keys for a mode, as after any other fail-open.
//
// The nodes are opened here rather than at construction, because udev creates
// them a moment after the uinput device, and the pointer proxy exists only once
// a keyboard that needs it is grabbed.
func (p *evdevProxy) probeOwnDevices() {
	if !p.forwarding.Load() {
		return
	}

	if p.keyboardNode == nil {
		p.keyboardNode = openProxyNode(p.uinputFd)
	}

	if pointer := p.capture.pointer.Load(); p.pointerNode == nil && pointer != nil {
		p.pointerNode = openProxyNode(pointer.fd)
	}

	for _, node := range []*proxyNode{p.keyboardNode, p.pointerNode} {
		if node != nil && p.heldByAnother(node) {
			p.failOpen(errWaylandEvdevProxyGrabbed)

			return
		}
	}
}

// refreshKeymap picks up a keymap the compositor replaced and, since the state
// under a new keymap has no key history, feeds it the keys the user is
// holding, so a modifier held across a layout change keeps choosing the level.
func (p *evdevProxy) refreshKeymap() {
	if !p.capture.pollKeymap() {
		return
	}

	for code := range p.global.pressed {
		p.capture.feedKey(code, true)
	}
}

// handle applies the forward rule to one event and hands it to the consumer.
//
// Keys, scan codes and sync reports are re-emitted on the proxy keyboard.
// Relative motion and buttons are not keys: they bypass the rule, the matcher
// and the session and go straight to the pointer proxy, with the sync report
// that closes their frame; a button the pointer proxy does not advertise goes
// nowhere (isMouseButton). LED events come back to the physical device from the
// compositor by way of the proxy (ledLoop), and re-emitting the device's own
// echo of them would send each one round again.
func (p *evdevProxy) handle(event waylandEvdevEvent) {
	switch event.eventType {
	case evdevEventKey:
		if isPointerButton(event.code) {
			if isMouseButton(event.code) {
				p.emitPointer(event)
			}

			return
		}

		p.handleKey(event)
	case evdevEventRel:
		p.emitPointer(event)
	case evdevEventSyn:
		p.emit(event)

		if p.pointerFrame {
			p.pointerFrame = false
			p.emitPointer(event)
		}
	case evdevEventMsc:
		p.emit(event)
	}
}

func (p *evdevProxy) handleKey(event waylandEvdevEvent) {
	code := event.code
	modifier := p.capture.modifierName(code)

	switch event.value {
	case evdevValuePress:
		p.capture.feedKey(code, true)
		p.trackGlobal(code, true)

		// A chord the idle matcher takes is withheld, so the focused app never
		// sees the activation chord — what the macOS tap does by consuming
		// the event. The match runs before the forward decision for that.
		withhold := p.session != nil
		if !withhold && modifier == "" && p.matchHotkey(code) {
			withhold = true
		}

		forwarded := p.rule.press(code, withhold)
		if forwarded {
			p.emit(event)
		}

		if p.session != nil {
			p.session.handlePress(code, modifier, forwarded)
		}
	case evdevValueRepeat:
		forwarded := p.rule.repeat(code)
		if forwarded {
			p.emit(event)
		}

		if p.session != nil {
			p.session.handleRepeat(code, modifier, forwarded)
		}
	case evdevValueRelease:
		p.capture.feedKey(code, false)
		p.trackGlobal(code, false)

		forwarded := p.rule.release(code)
		if forwarded {
			p.emit(event)
		}

		if p.session != nil {
			p.session.handleRelease(code, modifier, forwarded)
		}
	}
}

// trackGlobal keeps the proxy's own picture of the keyboard, which is the
// kernel's: every press and release on every device, whoever consumed it. It is
// what the idle matcher spells chords with, and what a passthrough reads to
// know which modifiers the user is physically holding.
func (p *evdevProxy) trackGlobal(code uint16, isDown bool) {
	if modifier := p.capture.modifierName(code); modifier != "" {
		p.global.trackModifier(code, modifier, isDown)

		return
	}

	p.global.trackKey(code, isDown)
}

// matchHotkey fires the binding for the chord a press completes, if there is
// one, and reports whether it did.
func (p *evdevProxy) matchHotkey(code uint16) bool {
	bindings := *p.bindings.Load()
	if len(bindings) == 0 {
		return false
	}

	key := p.capture.keyName(code)
	if key == "" {
		return false
	}

	signature := canonicalChordSignature(p.global.modifiers.prefix() + key)
	if signature == "" {
		return false
	}

	callback := bindings[signature]
	if callback == nil {
		return false
	}

	p.logger.Debug("Global hotkey matched", zap.String("chord", signature))

	go callback()

	return true
}

// emit re-emits one event on the proxy keyboard.
func (p *evdevProxy) emit(event waylandEvdevEvent) {
	var raw C.struct_input_event
	raw._type = C.ushort(event.eventType)
	raw.code = C.ushort(event.code)
	raw.value = C.int(event.value)

	p.write(p.uinputFd, &raw, 1)
}

// emitPointer re-emits one event on the pointer proxy. Without one nothing is
// emitted, which is the passive proxy, where nothing is grabbed and the motion
// reaches the compositor on its own device.
func (p *evdevProxy) emitPointer(event waylandEvdevEvent) {
	pointer := p.capture.pointer.Load()
	if pointer == nil {
		return
	}

	if event.eventType != evdevEventSyn {
		p.pointerFrame = true
	}

	var raw C.struct_input_event
	raw._type = C.ushort(event.eventType)
	raw.code = C.ushort(event.code)
	raw.value = C.int(event.value)

	p.write(pointer.fd, &raw, 1)
}

// emitKey re-emits a key event of the proxy's own making, with the sync report
// that makes it a complete frame.
func (p *evdevProxy) emitKey(code uint16, value int32) {
	var frame [2]C.struct_input_event
	frame[0]._type = C.ushort(evdevEventKey)
	frame[0].code = C.ushort(code)
	frame[0].value = C.int(value)
	frame[1]._type = C.ushort(evdevEventSyn)
	frame[1].code = C.ushort(evdevSynReport)

	p.write(p.uinputFd, &frame[0], len(frame))
}

// write puts count events on one proxy device, whole. Anything less means the
// device is gone from under the grab, and the grab is let go of (failOpen).
func (p *evdevProxy) write(fd C.int, events *C.struct_input_event, count int) {
	if !p.forwarding.Load() {
		return
	}

	written, err := C.neru_evdev_write_events(fd, events, C.int(count))
	if int(written) == count*int(unsafe.Sizeof(C.struct_input_event{})) {
		return
	}

	if err == nil {
		err = fmt.Errorf("%w: %d bytes for %d events", errWaylandEvdevShortWrite, written, count)
	}

	p.failOpen(err)
}

// failOpen releases the keyboards to the compositor and stops re-emitting, for
// the one failure that would otherwise take the user's keyboard with it: a
// proxy device that no longer accepts writes while the physical ones are held.
// Typing works again at once; what is lost is capturing keys for a mode, which
// a session asked for from here on is refused, and the tap falls back to the
// overlay's keyboard focus. The daemon has to restart to get the proxy back.
func (p *evdevProxy) failOpen(err error) {
	if !p.forwarding.CompareAndSwap(true, false) {
		return
	}

	p.capture.ungrabAll()

	// The keys now reach the compositor as well, so a session reading them
	// would hand the mode what the focused app also gets. Runs on the run
	// goroutine, the session's owner, because writes happen nowhere else.
	if p.session != nil {
		close(p.session.ended)
		p.session = nil
	}

	if errors.Is(err, errWaylandEvdevProxyGrabbed) {
		p.logger.Error(
			"Another process grabbed a neru proxy device, which a key remapper's device " +
				"auto-detect does; released the keyboards to the compositor so your keys and " +
				"remaps work, mode keyboard capture is off until the daemon restarts. Exclude " +
				"the neru- devices in the remapper's config (docs/LINUX_SETUP.md)",
		)

		return
	}

	p.logger.Error(
		"Keyboard proxy write failed; released the keyboards to the compositor, "+
			"mode keyboard capture is off until the daemon restarts",
		zap.Error(err),
	)
}

// forwardWithheld hands a press the session had withheld to the compositor
// after all, together with every modifier the user is physically holding that
// the session withheld too. Each is marked forwarded, so its repeats and its
// release follow it out under the ordinary rule: the app sees the modifier
// held for exactly as long as the user holds it, with no re-tapping and
// nothing to unwind when the mode ends.
func (p *evdevProxy) forwardWithheld(code uint16) {
	held := make([]uint16, 0, len(p.global.pressed))

	for pressed := range p.global.pressed {
		if pressed != code && p.capture.modifierName(pressed) != "" && !p.rule.isDown(pressed) {
			held = append(held, pressed)
		}
	}

	// Stable order, so two chords with the same modifiers reach the app the
	// same way.
	slices.Sort(held)

	for _, modifierCode := range held {
		p.rule.seed(modifierCode)
		p.emitKey(modifierCode, evdevValuePress)
	}

	p.rule.seed(code)
	p.emitKey(code, evdevValuePress)
}

// ledLoop carries the compositor's LED changes, which land on the proxy
// keyboard, to the lights on the physical ones. It ends when the proxy keyboard
// is destroyed, which is the process ending.
func (p *evdevProxy) ledLoop() {
	for {
		var event C.struct_input_event

		if C.neru_evdev_read_event(p.uinputFd, &event) <= 0 {
			return
		}

		if uint16(event._type) != evdevEventLed {
			continue
		}

		p.capture.writeToDevices(&event)

		var sync C.struct_input_event
		sync._type = C.ushort(evdevEventSyn)
		p.capture.writeToDevices(&sync)
	}
}
