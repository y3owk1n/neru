// Package accessibility provides an adapter for the macOS accessibility API.
//
// This package implements the ports.AccessibilityPort interface by wrapping
// the CGo/Objective-C bridge layer. It handles all conversion between domain
// models and infrastructure types, isolating the rest of the application from
// platform-specific details.
//
// # Architecture
//
//	┌─────────────────────────────────────┐
//	│    Application Services             │
//	│  (depends on AccessibilityPort)     │
//	└─────────────────────────────────────┘
//	              ↓
//	┌─────────────────────────────────────┐
//	│    Accessibility Adapter            │
//	│  (implements AccessibilityPort)     │
//	│  - Converts domain ↔ infra types    │
//	│  - Handles CGo complexity           │
//	└─────────────────────────────────────┘
//	              ↓
//	┌─────────────────────────────────────┐
//	│    Infrastructure Layer             │
//	│  - CGo bridge                       │
//	│  - Objective-C calls                │
//	└─────────────────────────────────────┘
//
// # Usage
//
//	adapter := accessibility.NewAdapter(
//		logger,
//		[]string{"com.apple.finder"}, // excluded bundles
//		[]string{"AXButton", "AXLink"}, // clickable roles
//	)
//
//	elements, err := adapter.GetClickableElements(ctx, filter)
//	if err != nil {
//		return err
//	}
//
// # Design Principles
//
//   - Isolation: CGo complexity is hidden behind clean interface
//   - Conversion: Handles all type conversions between layers
//   - Context-Aware: All operations respect context cancellation
//   - Error Handling: Converts infrastructure errors to domain errors
//   - Thread-Safe: Safe for concurrent use
package accessibility
