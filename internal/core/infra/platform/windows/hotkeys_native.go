//go:build windows

// internal/core/infra/platform/windows/hotkeys_native.go
// Global hotkey registration via RegisterHotKey on a dedicated message thread.
// Does not parse Neru config bindings.

package windows

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"
)

const wmHotkey = 0x0312

type hotkeyRegistration struct {
	id         int
	keyString  string
	modifiers  uint32
	virtualKey uint32
}

type hotkeyRegisterRequest struct {
	keyString  string
	modifiers  uint32
	virtualKey uint32
	callback   func()
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
	mu           sync.Mutex
	callbacks    map[int]func()
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
func GlobalHotkeyRegistry() (*HotkeyRegistry, error) {
	var initErr error

	globalHotkeyOnce.Do(func() {
		registry := &HotkeyRegistry{
			callbacks:    make(map[int]func()),
			registered:   make(map[int]hotkeyRegistration),
			threadStop:   make(chan struct{}),
			threadDone:   make(chan struct{}),
			registerCh:   make(chan hotkeyRegisterRequest),
			unregisterCh: make(chan hotkeyUnregisterRequest),
			threadReady:  make(chan struct{}),
			nextID:       1,
			logger:       zap.NewNop(),
		}

		if err := registry.start(); err != nil {
			initErr = err

			return
		}

		globalHotkeyRegistry = registry
	})

	if initErr != nil {
		return nil, initErr
	}

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

	r.logger = logger.Named("win32.hotkeys")
}

func (r *HotkeyRegistry) start() error {
	go r.messageLoop()

	return nil
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

			procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))

			continue
		}

		select {
		case req := <-r.registerCh:
			r.handleRegister(req)
		case req := <-r.unregisterCh:
			r.handleUnregister(req)
		case <-r.threadStop:
			return
		case <-time.After(10 * time.Millisecond):
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

	id := r.nextID
	r.nextID++

	// NULL hwnd posts WM_HOTKEY to this thread's queue; must match UnregisterHotKey.
	ret, _, regErr := procRegisterHotKey.Call(
		0,
		uintptr(id),
		uintptr(req.modifiers),
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

	r.callbacks[id] = req.callback
	r.registered[id] = hotkeyRegistration{
		id:         id,
		keyString:  req.keyString,
		modifiers:  req.modifiers,
		virtualKey: req.virtualKey,
	}

	r.logger.Info(
		"RegisterHotKey ok",
		zap.String("key", req.keyString),
		zap.Int("id", id),
		zap.Uint32("modifiers", req.modifiers),
		zap.Uint32("virtual_key", req.virtualKey),
	)

	req.resp <- hotkeyRegisterResponse{id: id}
}

func (r *HotkeyRegistry) handleUnregister(req hotkeyUnregisterRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	procUnregisterHotKey.Call(0, uintptr(req.id))
	delete(r.callbacks, req.id)
	delete(r.registered, req.id)
}

func (r *HotkeyRegistry) handleHotkeyMessage(id int) {
	r.mu.Lock()
	reg, hasReg := r.registered[id]
	callback := r.callbacks[id]
	r.mu.Unlock()

	if hasReg {
		r.logger.Info("WM_HOTKEY received", zap.String("key", reg.keyString), zap.Int("id", id))
	} else {
		r.logger.Warn("WM_HOTKEY received for unknown id", zap.Int("id", id))
	}

	if callback != nil {
		callback()
	}
}

// Register binds a hotkey string to a callback and returns a registry id.
func (r *HotkeyRegistry) Register(keyString string, callback func()) (int, error) {
	if callback == nil {
		return 0, fmt.Errorf("hotkey callback is nil")
	}

	mods, vk, err := ParseHotkeyString(keyString)
	if err != nil {
		return 0, err
	}

	select {
	case <-r.threadReady:
	case <-time.After(2 * time.Second):
		return 0, fmt.Errorf("hotkey message thread not ready")
	}

	resp := make(chan hotkeyRegisterResponse, 1)
	r.registerCh <- hotkeyRegisterRequest{
		keyString:  keyString,
		modifiers:  mods,
		virtualKey: vk,
		callback:   callback,
		resp:       resp,
	}

	result := <-resp

	return result.id, result.err
}

// Unregister removes a previously registered hotkey id.
func (r *HotkeyRegistry) Unregister(id int) {
	r.unregisterCh <- hotkeyUnregisterRequest{id: id}
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
