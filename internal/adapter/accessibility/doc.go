// Package accessibility provides an adapter for the macOS accessibility API.
//
// This package implements the ports.AccessibilityPort interface by wrapping
// the CGo/Objective-C bridge layer. It handles all conversion between domain
// models and infrastructure types, isolating the rest of the application from
// platform-specific details.
package accessibility
