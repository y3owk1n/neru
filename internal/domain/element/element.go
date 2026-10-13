package element

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// ID uniquely identifies an element.
type ID string

// Role is the accessibility role of an element.
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

// Subrole is the accessibility subrole of an element.
type Subrole string

// Common accessibility subroles.
const (
	SubroleMenuExtra Subrole = "AXMenuExtra"
)

// Source names what found an element, in the words --strategy takes, plus
// targets for the ones a caller supplied with --targets.
type Source string

// The sources an element can come from.
const (
	SourceAXTree  Source = "axtree"
	SourceVision  Source = "vision"
	SourceContour Source = "contour"
	SourceTargets Source = "targets"
)

// Element is a UI element in the accessibility tree (or detected
// via vision). It is immutable after creation to ensure thread safety.
//
// A new element is unfocused and has no identifier. One found on screen
// rather than in the accessibility tree keeps those defaults.
type Element struct {
	id          ID
	bounds      image.Rectangle
	role        Role
	isClickable bool
	title       string
	description string
	value       string
	searchText  string
	source      Source
	focused     bool
	identifier  string
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
		source: SourceAXTree,
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

// WithSource sets what found the element. IsVisionOnly reports true for any
// source but axtree.
func WithSource(source Source) Option {
	return func(e *Element) {
		e.source = source
	}
}

// WithFocused sets whether the element has keyboard focus.
func WithFocused(focused bool) Option {
	return func(e *Element) {
		e.focused = focused
	}
}

// WithIdentifier sets the identifier the app's developer gave the element.
func WithIdentifier(identifier string) Option {
	return func(e *Element) {
		e.identifier = identifier
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

// IsVisionOnly returns true if the element was found anywhere but the
// accessibility tree. Vision-only elements have no AX reference; actions must
// use coordinate-based clicks (PerformActionAtPoint).
func (e *Element) IsVisionOnly() bool {
	return e.source != SourceAXTree
}

// Source returns what found the element.
func (e *Element) Source() Source {
	return e.source
}

// IsFocused returns whether the element has keyboard focus.
func (e *Element) IsFocused() bool {
	return e.focused
}

// Identifier returns the identifier the app's developer gave the element.
func (e *Element) Identifier() string {
	return e.identifier
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
