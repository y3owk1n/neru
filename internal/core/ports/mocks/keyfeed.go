package mocks

import (
	"context"
	"sync"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// MockKeyFeedPort is a mock implementation of ports.KeyFeedPort.
type MockKeyFeedPort struct {
	FeedFunc func(context.Context, string) error

	mu       sync.Mutex
	fedKeys  []string
	feedCall int
}

// Feed implements ports.KeyFeedPort.
func (m *MockKeyFeedPort) Feed(ctx context.Context, key string) error {
	m.mu.Lock()
	m.fedKeys = append(m.fedKeys, key)
	m.feedCall++
	m.mu.Unlock()

	if m.FeedFunc != nil {
		return m.FeedFunc(ctx, key)
	}

	return nil
}

// FedKeys returns the keys passed to Feed, in order.
func (m *MockKeyFeedPort) FedKeys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.fedKeys...)
}

// FeedCallCount returns how many times Feed was called.
func (m *MockKeyFeedPort) FeedCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.feedCall
}

// Ensure MockKeyFeedPort implements ports.KeyFeedPort.
var _ ports.KeyFeedPort = (*MockKeyFeedPort)(nil)
