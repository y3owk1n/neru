// Package hotkeys provides comprehensive functionality for registering and handling global hotkeys
// in the Neru application using the Carbon Event Manager API, enabling system-wide keyboard shortcuts.
//
// This package implements a complete hotkey management system that allows users to define and
// trigger global keyboard shortcuts for various Neru functions. It serves as the primary interface
// between user input and application functionality, translating keyboard combinations into actions.
//
// The package includes both the low-level Manager for direct hotkey operations and an Adapter
// that implements the ports.HotkeyPort interface for integration with the application's
// port-based architecture.
package hotkeys
