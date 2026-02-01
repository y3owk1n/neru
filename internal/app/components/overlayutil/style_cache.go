package overlayutil

import (
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/core/infra/bridge"
)

// CachedStyle holds pointers to C strings for style properties.
// Fields are unsafe.Pointer to avoid C type dependency across packages.
type CachedStyle struct {
	FontFamily         unsafe.Pointer
	BgColor            unsafe.Pointer
	TextColor          unsafe.Pointer
	MatchedTextColor   unsafe.Pointer
	BorderColor        unsafe.Pointer
	MatchedBgColor     unsafe.Pointer
	MatchedBorderColor unsafe.Pointer
	HighlightColor     unsafe.Pointer
}

// StyleCache manages caching of C strings for styles to reduce allocations.
type StyleCache struct {
	mu    sync.RWMutex
	style CachedStyle
}

// NewStyleCache creates a new StyleCache.
func NewStyleCache() *StyleCache {
	return &StyleCache{}
}

// Get returns the cached style, calling updater if the cache is invalid (nil FontFamily).
// The updater function should populate the CachedStyle fields with new C strings.
// Existing strings are freed before calling updater.
func (c *StyleCache) Get(updater func(*CachedStyle)) CachedStyle {
	c.mu.RLock()

	if c.style.FontFamily == nil {
		c.mu.RUnlock()
		c.mu.Lock()
		// Double-check
		if c.style.FontFamily == nil {
			c.freeLocked()
			updater(&c.style)
		}

		c.mu.Unlock()
		c.mu.RLock()
	}
	defer c.mu.RUnlock()

	return c.style
}

// Free releases all cached C strings.
func (c *StyleCache) Free() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.freeLocked()
}

func (c *StyleCache) freeLocked() {
	bridge.FreeCString(c.style.FontFamily)
	c.style.FontFamily = nil
	bridge.FreeCString(c.style.BgColor)
	c.style.BgColor = nil
	bridge.FreeCString(c.style.TextColor)
	c.style.TextColor = nil
	bridge.FreeCString(c.style.MatchedTextColor)
	c.style.MatchedTextColor = nil
	bridge.FreeCString(c.style.BorderColor)
	c.style.BorderColor = nil
	bridge.FreeCString(c.style.MatchedBgColor)
	c.style.MatchedBgColor = nil
	bridge.FreeCString(c.style.MatchedBorderColor)
	c.style.MatchedBorderColor = nil
	bridge.FreeCString(c.style.HighlightColor)
	c.style.HighlightColor = nil
}
