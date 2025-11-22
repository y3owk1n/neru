// Package action defines domain models for user actions.
//
// This package provides types and utilities for representing actions
// that can be performed on UI elements, such as clicks, mouse movements,
// and scrolling.
//
// # Usage
//
//	actionType, err := action.ParseType("left_click")
//	if err != nil {
//		return err
//	}
//
//	if actionType.IsClick() {
//		// Handle click action
//	}
//
// # Design Principles
//
//   - Type Safety: Actions are represented as typed constants
//   - Validation: String parsing validates input
//   - No Dependencies: Pure domain logic with no external dependencies
package action
