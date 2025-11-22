// Package overlay provides an adapter for the UI overlay system.
//
// This package implements the ports.OverlayPort interface by wrapping
// the existing overlay.Manager. It handles conversion between domain
// models and UI types, providing a clean boundary between the application
// layer and the UI infrastructure.
//
// # Architecture
//
//	┌─────────────────────────────────────┐
//	│    Application Services             │
//	│  (depends on OverlayPort)           │
//	└─────────────────────────────────────┘
//	              ↓
//	┌─────────────────────────────────────┐
//	│    Overlay Adapter                  │
//	│  (implements OverlayPort)           │
//	│  - Converts domain ↔ UI types       │
//	│  - Context-aware operations         │
//	└─────────────────────────────────────┘
//	              ↓
//	┌─────────────────────────────────────┐
//	│    UI Layer                         │
//	│  - overlay.Manager                  │
//	│  - Platform-specific rendering      │
//	└─────────────────────────────────────┘
//
// # Usage
//
//	adapter := overlay.NewAdapter(overlayManager, logger)
//	err := adapter.ShowHints(ctx, hints)
//
// # Design Principles
//
//   - Thin Wrapper: Minimal logic, delegates to existing overlay.Manager
//   - Type Conversion: Handles domain → UI type conversion
//   - Context-Aware: All operations respect context cancellation
//   - Logging: Structured logging for observability
package overlay
