package element

import (
	"image"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// ID uniquely identifies an element.
type ID string

// Role represents the accessibility role of an element.
type Role string

// Common accessibility roles.
const (
	RoleButton             Role = "AXButton"
	RoleLink               Role = "AXLink"
	RoleTextField          Role = "AXTextField"
	RoleStaticText         Role = "AXStaticText"
	RoleImage              Role = "AXImage"
	RoleCheckBox           Role = "AXCheckBox"
	RoleRadioButton        Role = "AXRadioButton"
	RoleMenuItem           Role = "AXMenuItem"
	RoleMenuButton         Role = "AXMenuButton"
	RolePopUpButton        Role = "AXPopUpButton"
	RoleTabButton          Role = "AXTabButton"
	RoleSlider             Role = "AXSlider"
	RoleSwitch             Role = "AXSwitch"
	RoleDisclosureTriangle Role = "AXDisclosureTriangle"
	RoleTextArea           Role = "AXTextArea"
	RoleComboBox           Role = "AXComboBox"
	RolePopover            Role = "AXPopover"
	RoleSheet              Role = "AXSheet"
	RoleMenu               Role = "AXMenu"
	RoleSGTMenu            Role = "SGTMenu"
	RoleList               Role = "AXList"
	RoleHeading            Role = "AXHeading"
	RoleMenuBarItem        Role = "AXMenuBarItem"
	RoleMenuBar            Role = "AXMenuBar"
	RoleCell               Role = "AXCell"
	RoleRow                Role = "AXRow"
	RoleDockItem           Role = "AXDockItem"
	RoleIncrementor        Role = "AXIncrementor"
	RoleColorWell          Role = "AXColorWell"
	RoleSearchField        Role = "AXSearchField"
	RoleToolbarButton      Role = "AXToolbarButton"
	RoleToggle             Role = "AXToggle"
	RoleTable              Role = "AXTable"
	RoleOutline            Role = "AXOutline"
	RoleApplication        Role = "AXApplication"
	RoleWindow             Role = "AXWindow"
	RoleTabGroup           Role = "AXTabGroup"
	RoleGroup              Role = "AXGroup"
	RoleScrollArea         Role = "AXScrollArea"
	RoleSplitGroup         Role = "AXSplitGroup"
	RoleUnknown            Role = "AXUnknown"
	RoleGenericElement     Role = "AXGenericElement"
)

// Subrole represents the accessibility subrole of an element.
type Subrole string

// Common accessibility subroles.
const (
	SubroleMenuExtra Subrole = "AXMenuExtra"
)

// Element represents a UI element in the accessibility tree (or detected
// via vision). It is immutable after creation to ensure thread safety.
type Element struct {
	id          ID
	bounds      image.Rectangle
	role        Role
	isClickable bool
	title       string
	description string
	value       string
	searchText  string
	visionOnly  bool
	// stableID is a cross-scan identity (e.g. "pid:cfhash" on macOS), used to
	// carry a hint's label across auto-refreshes when the element persists. Empty
	// when no stable identity is available (the element then gets a fresh label).
	stableID string
}

// NewElement creates a new element with validation.
func NewElement(elementID ID, bounds image.Rectangle, role Role, opts ...Option) (*Element, error) {
	if elementID == "" {
		return nil, derrors.New(derrors.CodeInvalidInput, "element ID cannot be empty")
	}

	if bounds.Empty() {
		return nil, derrors.New(derrors.CodeInvalidInput, "element bounds cannot be empty")
	}

	element := &Element{
		id:     elementID,
		bounds: bounds,
		role:   role,
	}

	// Apply options
	for _, opt := range opts {
		opt(element)
	}

	return element, nil
}

// Option configures an Element.
type Option func(*Element)

// WithClickable sets whether the element is clickable.
func WithClickable(clickable bool) Option {
	return func(e *Element) {
		e.isClickable = clickable
	}
}

// WithTitle sets the element title.
func WithTitle(title string) Option {
	return func(e *Element) {
		e.title = title
	}
}

// WithDescription sets the element description.
func WithDescription(desc string) Option {
	return func(e *Element) {
		e.description = desc
	}
}

// WithValue sets the element value.
func WithValue(val string) Option {
	return func(e *Element) {
		e.value = val
	}
}

// WithSearchText sets additional searchable text associated with the element.
func WithSearchText(text string) Option {
	return func(e *Element) {
		e.searchText = text
	}
}

// WithStableID sets a cross-scan identity used for stable hint labels.
func WithStableID(id string) Option {
	return func(e *Element) {
		e.stableID = id
	}
}

// WithVisionOnly marks the element as detected via vision rather than
// the accessibility tree. Vision-only elements have no AX reference and
// actions must always use coordinate-based clicks (PerformActionAtPoint).
func WithVisionOnly() Option {
	return func(e *Element) {
		e.visionOnly = true
	}
}

// ID returns the element ID.
func (e *Element) ID() ID {
	return e.id
}

// Bounds returns the element bounds.
func (e *Element) Bounds() image.Rectangle {
	return e.bounds
}

// Role returns the element role.
func (e *Element) Role() Role {
	return e.role
}

// IsClickable returns whether the element is clickable.
func (e *Element) IsClickable() bool {
	return e.isClickable
}

// Title returns the element title.
func (e *Element) Title() string {
	return e.title
}

// Description returns the element description.
func (e *Element) Description() string {
	return e.description
}

// Value returns the element value.
func (e *Element) Value() string {
	return e.value
}

// SearchText returns additional searchable text associated with the element.
func (e *Element) SearchText() string {
	return e.searchText
}

// IsVisionOnly returns true if the element was detected via vision rather
// than the accessibility tree. Vision-only elements have no AX reference;
// actions must use coordinate-based clicks (PerformActionAtPoint).
func (e *Element) IsVisionOnly() bool {
	return e.visionOnly
}

// StableID returns the cross-scan identity, or "" if none is available.
func (e *Element) StableID() string {
	return e.stableID
}

// Center returns the center point of the element.
func (e *Element) Center() image.Point {
	return image.Point{
		X: e.bounds.Min.X + e.bounds.Dx()/2,
		Y: e.bounds.Min.Y + e.bounds.Dy()/2,
	}
}

// Contains checks if a point is within the element bounds.
func (e *Element) Contains(pt image.Point) bool {
	return pt.In(e.bounds)
}

// Overlaps checks if this element overlaps with another.
func (e *Element) Overlaps(other *Element) bool {
	return e.bounds.Overlaps(other.bounds)
}

// IsVisible checks if the element is visible within the given screen bounds.
func (e *Element) IsVisible(screenBounds image.Rectangle) bool {
	return e.bounds.Overlaps(screenBounds) && !e.bounds.Empty()
}
