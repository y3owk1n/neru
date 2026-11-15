//go:build darwin

package overlayutil

import "C"

import (
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// LabelCache keeps one C string per label, so a redraw hands the bridge the
// pointer it allocated last time instead of a new one. A pointer stays valid
// until Free. The zero value is ready to use.
type LabelCache struct {
	mu     sync.RWMutex
	labels map[string]unsafe.Pointer
}

// Get returns the C string for label, allocating it on first use. It returns
// an unsafe.Pointer because each cgo package has its own C types, so the
// caller converts it to its own *C.char.
func (c *LabelCache) Get(label string) unsafe.Pointer {
	c.mu.RLock()
	cached, found := c.labels[label]
	c.mu.RUnlock()

	if found {
		return cached
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Another caller may have allocated it between the two locks.
	if cached, found = c.labels[label]; found {
		return cached
	}

	if c.labels == nil {
		c.labels = make(map[string]unsafe.Pointer)
	}

	ptr := unsafe.Pointer(C.CString(label))
	c.labels[label] = ptr

	return ptr
}

// Free releases every cached C string. A label asked for afterwards is
// allocated again.
func (c *LabelCache) Free() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, ptr := range c.labels {
		darwin.FreeCString(ptr)
	}

	c.labels = nil
}
