//go:build darwin

package darwin

import (
	"reflect"
	"sync"
	"sync/atomic"
)

// cgoSlot holds a process-global target for C-exported callbacks. macOS bridges
// register a single Go handler, so tests and multiple App lifetimes share this
// slot. Each Set bumps a generation counter so sync and async dispatches can
// ignore events after clear (nil) or replacement.
type cgoSlot[T any] struct {
	mu     sync.RWMutex
	target T
	gen    atomic.Uint64
}

func (s *cgoSlot[T]) zero() T {
	var z T

	return z
}

func (s *cgoSlot[T]) isZero(v T) bool {
	return isEmptyValue(v)
}

// isEmptyValue reports whether v is unset. Typed nil interfaces are handled
// before reflect (ValueOf(nil) is invalid). Function types use IsNil; other
// scalars use IsZero.
func isEmptyValue[T any](v T) bool {
	if any(v) == nil {
		return true
	}

	reflected := reflect.ValueOf(v)
	if !reflected.IsValid() {
		return true
	}

	kind := reflected.Kind()
	if kind == reflect.Invalid {
		return true
	}

	if kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface ||
		kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice {
		return reflected.IsNil()
	}

	return reflected.IsZero()
}

// Set replaces the slot target and invalidates in-flight dispatches.
func (s *cgoSlot[T]) Set(target T) {
	s.mu.Lock()
	s.target = target
	s.gen.Add(1)
	s.mu.Unlock()
}

// Get returns the current target without generation checking.
func (s *cgoSlot[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.target
}

func (s *cgoSlot[T]) snapshot() (T, uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.isZero(s.target) {
		return s.zero(), 0, false
	}

	return s.target, s.gen.Load(), true
}

func (s *cgoSlot[T]) stillValid(gen uint64) bool {
	return s.gen.Load() == gen
}

// withValid invokes callback with the current target when the snapshot is still valid.
func (s *cgoSlot[T]) withValid(callback func(T)) {
	target, gen, ok := s.snapshot()
	if !ok || !s.stillValid(gen) {
		return
	}

	callback(target)
}

// withValidAsync invokes callback in a new goroutine when the snapshot is still valid.
func (s *cgoSlot[T]) withValidAsync(callback func(T)) {
	target, gen, ok := s.snapshot()
	if !ok || !s.stillValid(gen) {
		return
	}

	go func() {
		if !s.stillValid(gen) {
			return
		}

		callback(target)
	}()
}
