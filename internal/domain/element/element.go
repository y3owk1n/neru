package element

import (
	"image"

	derrors "github.com/y3owk1n/neru/internal/errors"
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
)

// Element represents a UI element in the accessibility tree.
// It is immutable after creation to ensure thread safety.
type Element struct {
	id          ID
	bounds      image.Rectangle
	role        Role
	isClickable bool
	title       string
	description string
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
