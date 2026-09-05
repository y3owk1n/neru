//go:build windows

package windows

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

// Global hotkey registration via RegisterHotKey on a dedicated message thread.
// Does not parse Neru config bindings.
const (
	wmHotkey = 0x0312

	// modNoRepeat is MOD_NOREPEAT: one WM_HOTKEY per press, however long the
	// key is held, instead of one per autorepeat. The hold is reported by the
	// release watcher instead, the way the macOS tap folds autorepeat.
	modNoRepeat = 0x4000

	hotkeyPollInterval   = 10 * time.Millisecond
	hotkeyThreadReadyTTL = 2 * time.Second
)

var (
	errHotkeyCallbackNil    = errors.New("hotkey callback is nil")
	errHotkeyThreadNotReady = errors.New("hotkey message thread not ready")
)

type hotkeyRegistration struct {
	id         int
	keyString  string
	modifiers  uint32
	virtualKey uint32
}

// hotkeyCallbacks is what one registration fires: press on WM_HOTKEY, and
// release (nil for Register) once the key reads up after it.
type hotkeyCallbacks struct {
	press   func()
	release func()
}

type hotkeyRegisterRequest struct {
	keyString  string
	modifiers  uint32
	virtualKey uint32
	callbacks  hotkeyCallbacks
	resp       chan hotkeyRegisterResponse
}

type hotkeyRegisterResponse struct {
	id  int
	err error
}

type hotkeyUnregisterRequest struct {
	id int
}

// HotkeyRegistry manages RegisterHotKey bindings on a dedicated message thread.
type HotkeyRegistry struct {
	mu        sync.Mutex
	callbacks map[int]hotkeyCallbacks
	// held is the stop channel of the release watcher for each hotkey whose
	// key is down, by registry id. A WM_HOTKEY for an id already here is the
	// key still held and fires nothing.
	held map[int]chan struct{}
	// keyDown reads whether a virtual key is down right now; a test stands in
	// for GetAsyncKeyState here.
	keyDown      func(vk uint32) bool
	threadDone   chan struct{}
	threadStop   chan struct{}
	registered   map[int]hotkeyRegistration
	nextID       int
	registerCh   chan hotkeyRegisterRequest
	unregisterCh chan hotkeyUnregisterRequest
	threadReady  chan struct{}
	logger       *zap.Logger
}

var (
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procPeekMessageW     = user32.NewProc("PeekMessageW")

	globalHotkeyRegistry *HotkeyRegistry
	globalHotkeyOnce     sync.Once
)

// GlobalHotkeyRegistry returns the process-wide hotkey registry.
//
// The error result is retained for API symmetry with other platforms; the
// Windows registry starts its message thread lazily and never fails here.
func GlobalHotkeyRegistry() (*HotkeyRegistry, error) {
	globalHotkeyOnce.Do(func() {
		registry := &HotkeyRegistry{
			callbacks:    make(map[int]hotkeyCallbacks),
			held:         make(map[int]chan struct{}),
			keyDown:      isVirtualKeyDown,
			registered:   make(map[int]hotkeyRegistration),
			threadStop:   make(chan struct{}),
			threadDone:   make(chan struct{}),
			registerCh:   make(chan hotkeyRegisterRequest),
			unregisterCh: make(chan hotkeyUnregisterRequest),
			threadReady:  make(chan struct{}),
			nextID:       1,
			logger:       zap.NewNop(),
		}

		registry.start()

		globalHotkeyRegistry = registry
	})

	return globalHotkeyRegistry, nil
}

// SetHotkeyRegistryLogger attaches a logger for hotkey diagnostics.
func (r *HotkeyRegistry) SetHotkeyRegistryLogger(logger *zap.Logger) {
	if r == nil {
		return
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	r.logger = logger.Named("hotkeys.win32")
}

// Register binds a hotkey string to a callback and returns a registry id.
func (r *HotkeyRegistry) Register(keyString string, callback func()) (int, error) {
	return r.RegisterWithRelease(keyString, callback, nil)
}

// RegisterWithRelease binds a hotkey string to a press callback and an optional
// release callback, and returns a registry id.
//
// RegisterHotKey reports the press only, so the release is found by reading
// the key's state (GetAsyncKeyState) every hotkeyPollInterval from the
// WM_HOTKEY until it reads up. The poll runs only while a hotkey is held, which
// is why it is preferred over the other source of releases, the WH_KEYBOARD_LL
// hook: that hook sits on every keystroke for as long as it is installed, and
// installing it for the daemon's lifetime would put a hook procedure on the
// idle path to pay for a release that only a held hotkey needs.
func (r *HotkeyRegistry) RegisterWithRelease(keyString string, press, release func()) (int, error) {
	if press == nil {
		return 0, errHotkeyCallbackNil
	}

	mods, virtualKey, err := ParseHotkeyString(keyString)
	if err != nil {
		return 0, err
	}

	select {
	case <-r.threadReady:
	case <-time.After(hotkeyThreadReadyTTL):
		return 0, errHotkeyThreadNotReady
	}

	resp := make(chan hotkeyRegisterResponse, 1)
	r.registerCh <- hotkeyRegisterRequest{
		keyString:  keyString,
		modifiers:  mods,
		virtualKey: virtualKey,
		callbacks:  hotkeyCallbacks{press: press, release: release},
		resp:       resp,
	}

	result := <-resp

	return result.id, result.err
}

// Unregister removes a previously registered hotkey id.
func (r *HotkeyRegistry) Unregister(hotkeyID int) {
	r.unregisterCh <- hotkeyUnregisterRequest{id: hotkeyID}
}

// UnregisterAll removes all hotkeys.
func (r *HotkeyRegistry) UnregisterAll() {
	r.mu.Lock()

	ids := make([]int, 0, len(r.registered))
	for id := range r.registered {
		ids = append(ids, id)
	}
	r.mu.Unlock()

	for _, id := range ids {
		r.Unregister(id)
	}
}

func (r *HotkeyRegistry) start() {
	go r.messageLoop()
}

func (r *HotkeyRegistry) messageLoop() {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()
	defer close(r.threadDone)

	close(r.threadReady)
	r.logger.Info("hotkey message thread started")

	var message msg
	for {
		r.drainPendingOps()

		ret, _, _ := procPeekMessageW.Call(
			uintptr(unsafe.Pointer(&message)),
			0,
			0,
			0,
			pmRemove,
		)
		if ret != 0 {
			if message.message == wmQuit {
				return
			}

			if message.message == wmHotkey {
				r.handleHotkeyMessage(int(message.wParam))

				continue
			}

			discardCall(procTranslateMessage.Call(uintptr(unsafe.Pointer(&message))))
			discardCall(procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message))))

			continue
		}

		select {
		case req := <-r.registerCh:
			r.handleRegister(req)
		case req := <-r.unregisterCh:
			r.handleUnregister(req)
		case <-r.threadStop:
			return
		case <-time.After(hotkeyPollInterval):
		}
	}
}

func (r *HotkeyRegistry) drainPendingOps() {
	for {
		select {
		case req := <-r.registerCh:
			r.handleRegister(req)
		case req := <-r.unregisterCh:
			r.handleUnregister(req)
		case <-r.threadStop:
			return
		default:
			return
		}
	}
}

func (r *HotkeyRegistry) handleRegister(req hotkeyRegisterRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hotkeyID := r.nextID
	r.nextID++

	// NULL hwnd posts WM_HOTKEY to this thread's queue; must match UnregisterHotKey.
	ret, _, regErr := procRegisterHotKey.Call(
		0,
		uintptr(hotkeyID),
		uintptr(req.modifiers|modNoRepeat),
		uintptr(req.virtualKey),
	)
	if ret == 0 {
		r.logger.Error(
			"RegisterHotKey failed",
			zap.String("key", req.keyString),
			zap.Uint32("modifiers", req.modifiers),
			zap.Uint32("virtual_key", req.virtualKey),
			zap.Error(regErr),
		)

		req.resp <- hotkeyRegisterResponse{
			err: fmt.Errorf("RegisterHotKey: %w", regErr),
		}

		return
	}

	r.callbacks[hotkeyID] = req.callbacks
	r.registered[hotkeyID] = hotkeyRegistration{
		id:         hotkeyID,
		keyString:  req.keyString,
		modifiers:  req.modifiers,
		virtualKey: req.virtualKey,
	}

	r.logger.Info(
		"RegisterHotKey ok",
		zap.String("key", req.keyString),
		zap.Int("id", hotkeyID),
		zap.Uint32("modifiers", req.modifiers),
		zap.Uint32("virtual_key", req.virtualKey),
	)

	req.resp <- hotkeyRegisterResponse{id: hotkeyID}
}

func (r *HotkeyRegistry) handleUnregister(req hotkeyUnregisterRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	discardCall(procUnregisterHotKey.Call(0, uintptr(req.id)))
	delete(r.callbacks, req.id)
	delete(r.registered, req.id)

	// A hotkey unregistered while held owes no release: the binder stops every
	// repeat before it unregisters.
	if stop, held := r.held[req.id]; held {
		close(stop)
		delete(r.held, req.id)
	}
}

func (r *HotkeyRegistry) handleHotkeyMessage(hotkeyID int) {
	r.mu.Lock()
	reg, hasReg := r.registered[hotkeyID]
	callbacks := r.callbacks[hotkeyID]
	_, held := r.held[hotkeyID]

	var stop chan struct{}

	if hasReg && !held && callbacks.release != nil {
		stop = make(chan struct{})
		r.held[hotkeyID] = stop
	}
	r.mu.Unlock()

	if !hasReg {
		r.logger.Warn("WM_HOTKEY received for unknown id", zap.Int("id", hotkeyID))

		return
	}

	r.logger.Debug(
		"WM_HOTKEY received",
		zap.String("key", reg.keyString),
		zap.Int("id", hotkeyID),
	)

	// The key is still down from the press that started the watcher, so this
	// is an autorepeat MOD_NOREPEAT did not fold, and the hold is already
	// being reported.
	if held {
		return
	}

	if callbacks.press != nil {
		callbacks.press()
	}

	if stop != nil {
		go r.watchRelease(hotkeyID, reg.virtualKey, stop, callbacks.release)
	}
}

// watchRelease reads the key every hotkeyPollInterval until it is up, then
// fires release, unless stop closes first (the hotkey was unregistered).
func (r *HotkeyRegistry) watchRelease(
	hotkeyID int,
	virtualKey uint32,
	stop chan struct{},
	release func(),
) {
	ticker := time.NewTicker(hotkeyPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		if r.keyDown(virtualKey) {
			continue
		}

		// The hold ends under the lock before release runs, so the next
		// press's WM_HOTKEY finds the id free.
		r.mu.Lock()
		if r.held[hotkeyID] == stop {
			delete(r.held, hotkeyID)
		}
		r.mu.Unlock()

		release()

		return
	}
}
