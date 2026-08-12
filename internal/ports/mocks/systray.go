package mocks

import (
	"sync"

	"github.com/y3owk1n/neru/internal/ports"
)

// MockSystrayPort is an in-memory implementation of ports.SystrayPort.
//
// It builds a real menu tree so a test can assert on the items a component
// created, and every item carries a working click channel that Click can fire.
type MockSystrayPort struct {
	mu             sync.Mutex
	title          string
	tooltip        string
	icon           []byte
	iconIsTemplate bool
	items          []*MockSystrayMenuItem
}

// SetTitle implements ports.SystrayPort.
func (m *MockSystrayPort) SetTitle(title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.title = title
}

// SetTooltip implements ports.SystrayPort.
func (m *MockSystrayPort) SetTooltip(tooltip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tooltip = tooltip
}

// SetIcon implements ports.SystrayPort.
func (m *MockSystrayPort) SetIcon(icon []byte, template bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.icon = icon
	m.iconIsTemplate = template
}

// AddMenuItem implements ports.SystrayPort.
func (m *MockSystrayPort) AddMenuItem(title string) ports.SystrayMenuItem {
	item := newMockSystrayMenuItem(title)

	m.mu.Lock()
	m.items = append(m.items, item)
	m.mu.Unlock()

	return item
}

// AddSeparator implements ports.SystrayPort.
func (m *MockSystrayPort) AddSeparator() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = append(m.items, newMockSystrayMenuItem(""))
}

// Title returns the last value passed to SetTitle.
func (m *MockSystrayPort) Title() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.title
}

// Tooltip returns the last value passed to SetTooltip.
func (m *MockSystrayPort) Tooltip() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.tooltip
}

// Icon returns the last icon passed to SetIcon and whether it was set as a
// macOS template image.
func (m *MockSystrayPort) Icon() ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.icon, m.iconIsTemplate
}

// Items returns the top-level items created so far.
func (m *MockSystrayPort) Items() []*MockSystrayMenuItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]*MockSystrayMenuItem(nil), m.items...)
}

// FindItem returns the first top-level item with the given title, or nil.
func (m *MockSystrayPort) FindItem(title string) *MockSystrayMenuItem {
	for _, item := range m.Items() {
		if item.Title() == title {
			return item
		}
	}

	return nil
}

// MockSystrayMenuItem is an in-memory ports.SystrayMenuItem.
type MockSystrayMenuItem struct {
	clicked chan struct{}

	mu       sync.Mutex
	title    string
	disabled bool
	checked  bool
	hidden   bool
	children []*MockSystrayMenuItem
}

func newMockSystrayMenuItem(title string) *MockSystrayMenuItem {
	return &MockSystrayMenuItem{
		title:   title,
		clicked: make(chan struct{}, 1),
	}
}

// Clicked implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) Clicked() <-chan struct{} { return m.clicked }

// Click simulates the user selecting this item. It drops the event when one is
// already pending, matching the real backends' buffered-by-one channels.
func (m *MockSystrayMenuItem) Click() {
	select {
	case m.clicked <- struct{}{}:
	default:
	}
}

// SetTitle implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) SetTitle(title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.title = title
}

// Title returns the item's current label.
func (m *MockSystrayMenuItem) Title() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.title
}

// Enable implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabled = false
}

// Disable implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disabled = true
}

// Disabled reports whether the item is currently disabled.
func (m *MockSystrayMenuItem) Disabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.disabled
}

// Check implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) Check() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checked = true
}

// Uncheck implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) Uncheck() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checked = false
}

// Checked implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) Checked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.checked
}

// Show implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) Show() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hidden = false
}

// Hide implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) Hide() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hidden = true
}

// Hidden reports whether the item is currently hidden.
func (m *MockSystrayMenuItem) Hidden() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.hidden
}

// AddSubMenuItem implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) AddSubMenuItem(title string) ports.SystrayMenuItem {
	child := newMockSystrayMenuItem(title)

	m.mu.Lock()
	m.children = append(m.children, child)
	m.mu.Unlock()

	return child
}

// AddSeparator implements ports.SystrayMenuItem.
func (m *MockSystrayMenuItem) AddSeparator() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.children = append(m.children, newMockSystrayMenuItem(""))
}

// Children returns this item's submenu entries.
func (m *MockSystrayMenuItem) Children() []*MockSystrayMenuItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]*MockSystrayMenuItem(nil), m.children...)
}

// Ensure the mocks satisfy the port.
var (
	_ ports.SystrayPort     = (*MockSystrayPort)(nil)
	_ ports.SystrayMenuItem = (*MockSystrayMenuItem)(nil)
)
