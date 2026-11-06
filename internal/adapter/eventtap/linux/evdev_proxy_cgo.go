// This is the evdev keyboard proxy (ADR 0014). Every physical keyboard is held
// (EVIOCGRAB) for the daemon's lifetime and re-emitted through one uinput
// keyboard, so libinput only sees a device Neru controls. Capturing keys for a
// mode is then a routing decision on the run goroutine, like the macOS
// CGEventTap, not a grab acquired on every activation.
//
// One goroutine reads every device, applies the forward rule, and hands each
// event to the mode session while one is open, the hotkey matcher otherwise.
// Exactly one of the two sees any press, so one press never runs a binding
// twice.

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
	// opened. The proxy is then passive. Devices are read without a grab,
	// hotkeys still match, and a mode session is refused.
	uinputFd C.int

	// forwarding is whether keys are re-emitted on uinputFd. It goes false
	// for good when a write fails (failOpen), the one way the proxy can lose
	// the keyboard after grabbing it.
	forwarding atomic.Bool

	// Owned by the run goroutine.
	rule    forwardRule
	global  waylandEvdevKeyState
	session *evdevSession

	// lifted is the modifiers released on the proxy keyboard while the user
	// still holds them (liftHeldModifiers). Their physical release is
	// swallowed, and restoreLiftedModifiers presses the rest again.
	lifted map[uint16]bool

	// pointerFrame is whether the current frame put anything on the pointer
	// proxy, so its sync report goes there too.
	pointerFrame bool

	// keyboardNode and pointerNode are the proxy's own event nodes, opened on
	// the run goroutine once udev creates them and probed for another
	// process's grab (probeOwnDevices). heldByAnother is a test seam.
	keyboardNode  *proxyNode
	pointerNode   *proxyNode
	heldByAnother func(*proxyNode) bool

	// emitted sees every key event the proxy makes up, as opposed to one it
	// re-emits. A test's record of what reached the compositor.
	emitted func(code uint16, value int32)

	bindings atomic.Pointer[map[string]hotkeyBinding]

	// heldHotkeys is the release callback owed for each still-down key whose
	// press matched a chord. Owned by the run goroutine. The press fires once
	// at match time, and the release when the key comes up, whichever
	// consumer got the events in between.
	heldHotkeys map[uint16]func()

	// dispatch runs the matched bindings' callbacks in order, off this
	// goroutine.
	dispatch *HotkeyDispatcher

	control chan proxyCommand
	done    chan struct{}
}

// proxyCommand replaces the current session (nil ends it), or with run set
// runs that on the run goroutine. ack closes once the replacement is in force
// or run has returned, so no event reaches the old session afterwards.
type proxyCommand struct {
	session *evdevSession
	run     func()
	ack     chan struct{}
}

// hotkeyBinding is what the idle matcher fires for a chord: press when the
// chord's key goes down, release (which may be nil) when that key comes up.
type hotkeyBinding struct {
	press   func()
	release func()
}

var (
	sharedProxyMu sync.Mutex
	sharedProxy   *evdevProxy
)

// acquireEvdevProxy returns the process-wide proxy, building it on first use.
// A failed build can be retried, since joining the input group later fixes
// it. Nothing closes the proxy. The kernel drops every grab and the uinput
// device when the daemon exits, crashed or not.
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
// before any key that could be down at a mode's activation. Built lazily, a
// mode launched from a compositor binding found its modifier still down, the
// device waited for it to come up (waitIdle), and the session was refused.
func WarmEvdevProxy(logger *zap.Logger) error {
	_, err := acquireEvdevProxy(logger)

	return err
}

func newEvdevProxy(logger *zap.Logger) (*evdevProxy, error) {
	var uinputFd C.int = -1

	created, errno := C.neru_uinput_create_proxy_keyboard(&uinputFd)
	if created == 0 {
		// Warn, not error, because hotkeys still work. Mode key capture does
		// not, and the tap says so when a mode asks.
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
		heldHotkeys:   make(map[uint16]func()),
		lifted:        make(map[uint16]bool),
		dispatch:      NewHotkeyDispatcher(),
	}

	proxy.forwarding.Store(uinputFd >= 0)

	empty := map[string]hotkeyBinding{}
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

// setBindings replaces the chords the idle matcher fires on. The run
// goroutine reads the map without a lock, so it is swapped whole.
func (p *evdevProxy) setBindings(bindings map[string]hotkeyBinding) {
	copied := make(map[string]hotkeyBinding, len(bindings))
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

	err := p.capture.beginSession()
	if err != nil {
		return err
	}

	err = p.send(proxyCommand{session: session, ack: make(chan struct{})})
	if err != nil {
		p.capture.endSession()

		return err
	}

	return nil
}

// stopSession returns keys to the compositor and the idle matcher, and returns
// once no event can reach the session any more.
func (p *evdevProxy) stopSession() {
	_ = p.send(proxyCommand{ack: make(chan struct{})})
	p.capture.endSession()
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
			if cmd.run != nil {
				cmd.run()
				close(cmd.ack)

				continue
			}

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
// proxy's own devices. A key remapper's auto-detect does that to a new node,
// and with the remapper's output grabbed here, every key circles between the
// two processes and the keyboard is dead. Letting go returns the remapper's
// output to the compositor. Mode key capture is lost, as after any fail-open.
//
// The nodes are opened here, not at construction, because udev creates them a
// moment after the uinput device, and the pointer proxy exists only once a
// keyboard needs it.
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
// Relative motion and buttons bypass the rule, the matcher and the session,
// and go to the pointer proxy with the sync report closing their frame. A
// button the pointer proxy does not advertise goes nowhere (isMouseButton).
// LED events are dropped, because the compositor's LED changes already reach
// the device through ledLoop and re-emitting the echo would loop.
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

		// A chord the idle matcher takes is withheld, as the macOS tap consumes
		// it, so the match runs before the forward decision.
		//
		// A modifier is never withheld, so a compositor bind on a bare
		// modifier keeps firing while a mode is open (#1665). The session
		// still reads it for its sticky toggle.
		withhold := modifier == "" && (p.session != nil || p.matchHotkey(code))

		forwarded := p.rule.press(code, withhold)
		if forwarded {
			// A second keyboard pressing a lifted modifier puts it back
			// down for the compositor, so it is lifted no longer.
			delete(p.lifted, code)
			p.emit(event)
		} else if modifier == "" && !p.global.modifiers.allZero() {
			p.cancelModifierShortcut()
		}

		if p.session != nil {
			p.session.handlePress(code, modifier, forwarded)
		}
	case evdevValueRepeat:
		forwarded := p.rule.repeat(code)
		if forwarded && !p.lifted[code] {
			p.emit(event)
		}

		if p.session != nil {
			p.session.handleRepeat(code, modifier, forwarded)
		}
	case evdevValueRelease:
		p.capture.feedKey(code, false)
		p.trackGlobal(code, false)
		p.releaseHotkey(code)

		forwarded := p.rule.release(code)
		if forwarded {
			// The compositor already saw a lifted modifier go up.
			if p.lifted[code] {
				delete(p.lifted, code)
			} else {
				p.emit(event)
			}
		}

		if p.session != nil {
			p.session.handleRelease(code, modifier, forwarded)
		}
	}
}

// cancelModifierShortcut stands in for a withheld press under a held modifier.
// KWin, Mutter and Hyprland release binds fire on a modifier pressed and
// released alone. The compositor saw the modifier's press and will see its
// release, but not the key between, so it gets a tap of KEY_UNKNOWN, which
// carries no symbol and reaches no application.
func (p *evdevProxy) cancelModifierShortcut() {
	p.emitKey(evdevKeyUnknown, evdevValuePress)
	p.emitKey(evdevKeyUnknown, evdevValueRelease)
}

// trackGlobal records every press and release on every device, whoever
// consumed it, as the proxy's own picture of the keyboard. The idle matcher spells
// chords with it, and a passthrough reads physically held modifiers from it.
func (p *evdevProxy) trackGlobal(code uint16, isDown bool) {
	if modifier := p.capture.modifierName(code); modifier != "" {
		p.global.trackModifier(code, modifier, isDown)

		return
	}

	p.global.trackKey(code, isDown)
}

// matchHotkey fires the binding for the chord a press completes, if any, and
// reports whether it did. A held key arrives as repeats, so it fires once.
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

	binding, bound := bindings[signature]
	if !bound || binding.press == nil {
		return false
	}

	p.logger.Debug("Global hotkey matched", zap.String("chord", signature))

	if binding.release != nil {
		p.heldHotkeys[code] = binding.release
	}

	p.dispatch.Dispatch(binding.press)

	return true
}

// releaseHotkey fires the release owed for a key whose press matched a chord.
// It is keyed by key alone, because the release is owed whether or not the
// modifier is still down.
func (p *evdevProxy) releaseHotkey(code uint16) {
	release, held := p.heldHotkeys[code]
	if !held {
		return
	}

	delete(p.heldHotkeys, code)

	p.dispatch.Dispatch(release)
}

// emit re-emits one event on the proxy keyboard.
func (p *evdevProxy) emit(event waylandEvdevEvent) {
	var raw C.struct_input_event
	raw._type = C.ushort(event.eventType)
	raw.code = C.ushort(event.code)
	raw.value = C.int(event.value)

	p.write(p.uinputFd, &raw, 1)
}

// emitPointer re-emits one event on the pointer proxy. Without one (the
// passive proxy) nothing is emitted, since the device is not grabbed.
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
	if p.emitted != nil {
		p.emitted(code, value)
	}

	frame := keyFrame(code, value)
	p.write(p.uinputFd, &frame[0], len(frame))
}

// keyFrame is one key event and the sync report that completes it.
func keyFrame(code uint16, value int32) [2]C.struct_input_event {
	var frame [2]C.struct_input_event
	frame[0]._type = C.ushort(evdevEventKey)
	frame[0].code = C.ushort(code)
	frame[0].value = C.int(value)
	frame[1]._type = C.ushort(evdevEventSyn)
	frame[1].code = C.ushort(evdevSynReport)

	return frame
}

// write puts count events on one proxy device, whole. A short write means the
// device is gone, and the grab is let go (failOpen).
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

// failOpen releases the keyboards to the compositor and stops re-emitting
// when a proxy device stops accepting writes while the physical ones are
// held. Typing works at once. Mode key capture is lost, so later sessions are
// refused and the tap falls back to overlay focus. Only a restart restores
// the proxy.
func (p *evdevProxy) failOpen(err error) {
	if !p.forwarding.CompareAndSwap(true, false) {
		return
	}

	p.releaseForwarded()

	p.capture.ungrabAll()

	// The keys now reach the compositor too, so the session must end. This
	// runs on the run goroutine, the session's owner and the only writer.
	if p.session != nil {
		close(p.session.ended)
		p.session = nil
		p.capture.endSession()
	}

	if errors.Is(err, errWaylandEvdevProxyGrabbed) {
		p.logger.Error(
			"Another process grabbed a neru proxy device, which a key remapper's device " +
				"auto-detect does; released the keyboards to the compositor so your keys and " +
				"remaps work, mode keyboard capture is off until the daemon restarts. Exclude " +
				"the neru- devices in the remapper's config (docs/guide/linux.md)",
		)

		return
	}

	p.logger.Error(
		"Keyboard proxy write failed; released the keyboards to the compositor, "+
			"mode keyboard capture is off until the daemon restarts",
		zap.Error(err),
	)
}

// releaseForwarded re-emits a release on the proxy keyboard for every key the
// rule counts as forwarded and down, then zeroes the rule. Failing open is the
// one time a keyboard is let go without waiting for idle. A key down then
// would get its release on the physical device, whose libinput never saw the
// press, and stay down on the proxy for good (Hyprland would put that Super
// on every later key). Base keys go up before modifiers. The writes bypass
// the cleared forwarding flag, and on a dead proxy they fail.
func (p *evdevProxy) releaseForwarded() {
	var keys, modifiers []uint16

	for code := range uint16(evdevKeyCodeCount) {
		switch {
		case !p.rule.isDown(code):
		case p.capture.modifierName(code) != "":
			modifiers = append(modifiers, code)
		default:
			keys = append(keys, code)
		}
	}

	p.rule = forwardRule{}

	for _, code := range append(keys, modifiers...) {
		frame := keyFrame(code, evdevValueRelease)
		C.neru_evdev_write_events(p.uinputFd, &frame[0], C.int(len(frame)))
	}
}

// forwardWithheld hands a press the session withheld to the compositor after
// all, with every physically held modifier the session also withheld. Each is
// marked forwarded, so its repeats and release follow under the ordinary
// rule, and nothing needs unwinding when the mode ends.
func (p *evdevProxy) forwardWithheld(code uint16) {
	held := make([]uint16, 0, len(p.global.pressed))

	for pressed := range p.global.pressed {
		if pressed != code && p.capture.modifierName(pressed) != "" && !p.rule.isDown(pressed) {
			held = append(held, pressed)
		}
	}

	// Stable order, so equal modifier sets reach the app the same way.
	slices.Sort(held)

	for _, modifierCode := range held {
		p.rule.seed(modifierCode)
		p.emitKey(modifierCode, evdevValuePress)
	}

	p.rule.seed(code)
	p.emitKey(code, evdevValuePress)
}

// liftHeldModifiers releases, on the proxy keyboard, every modifier the user
// physically holds that the compositor has down, and reports whether it
// released any. A Wayland pointer event carries the seat's held modifiers, so
// a hotkey chord still held during an injected click makes it a modified
// click (footnote 7 of docs/contributing/platform-internals.md). Only the
// proxy can fix that on Wayland. Runs on the run goroutine.
func (p *evdevProxy) liftHeldModifiers() bool {
	held := make([]uint16, 0, len(p.global.pressed))

	for pressed := range p.global.pressed {
		if p.capture.modifierName(pressed) != "" && p.rule.isDown(pressed) && !p.lifted[pressed] {
			held = append(held, pressed)
		}
	}

	if len(held) == 0 {
		return false
	}

	slices.Sort(held)

	for _, code := range held {
		p.lifted[code] = true
		p.emitKey(code, evdevValueRelease)
	}

	return true
}

// restoreLiftedModifiers presses again the lifted modifiers the user still
// holds. The re-press is followed by the symbol-less tap of
// cancelModifierShortcut, because otherwise the modifier reads as pressed and
// released alone, a launcher shortcut on KWin, Mutter and Hyprland.
// Runs on the run goroutine.
func (p *evdevProxy) restoreLiftedModifiers() {
	if len(p.lifted) == 0 {
		return
	}

	held := make([]uint16, 0, len(p.lifted))

	for code := range p.lifted {
		if p.rule.isDown(code) {
			held = append(held, code)
		}
	}

	clear(p.lifted)

	if len(held) == 0 {
		return
	}

	slices.Sort(held)

	for _, code := range held {
		p.emitKey(code, evdevValuePress)
	}

	p.cancelModifierShortcut()
}

// LiftHeldModifiers releases the modifiers the user physically holds on the
// proxy keyboard, so the next injected pointer action carries only the
// modifiers it names, and reports whether any was released. Without a
// forwarding proxy there is nothing to lift. RestoreLiftedModifiers undoes it.
func LiftHeldModifiers() (bool, error) {
	proxy := currentEvdevProxy()
	if proxy == nil || !proxy.forwarding.Load() {
		return false, nil
	}

	var lifted bool

	err := proxy.send(proxyCommand{
		run: func() { lifted = proxy.liftHeldModifiers() },
		ack: make(chan struct{}),
	})

	return lifted, err
}

// RestoreLiftedModifiers presses again the modifiers LiftHeldModifiers
// released that the user still holds.
func RestoreLiftedModifiers() error {
	proxy := currentEvdevProxy()
	if proxy == nil || !proxy.forwarding.Load() {
		return nil
	}

	return proxy.send(proxyCommand{
		run: proxy.restoreLiftedModifiers,
		ack: make(chan struct{}),
	})
}

// currentEvdevProxy returns the proxy if one has been built. It never builds
// one, because a pointer action is no reason to grab the keyboards.
func currentEvdevProxy() *evdevProxy {
	sharedProxyMu.Lock()
	defer sharedProxyMu.Unlock()

	return sharedProxy
}

// ledLoop carries the compositor's LED changes from the proxy keyboard to the
// physical ones. It ends when the proxy keyboard is destroyed.
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
