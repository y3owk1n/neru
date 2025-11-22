// Package element provides domain models for UI elements.
//
// This package defines the core Element type which represents a UI element
// in the accessibility tree. Elements are immutable after creation to ensure
// thread safety and prevent accidental modification.
//
// # Usage
//
//	elem, err := element.NewElement(
//		element.ID("button-1"),
//		image.Rect(10, 10, 100, 50),
//		element.RoleButton,
//		element.WithClickable(true),
//		element.WithTitle("Submit"),
//	)
//	if err != nil {
//		return err
//	}
//
// # Design Principles
//
// - Immutability: Elements cannot be modified after creation
// - Validation: All elements are validated at construction time
// - No Dependencies: This package has no dependencies on infrastructure
package element
