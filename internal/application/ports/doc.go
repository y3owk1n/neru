// Package ports defines interfaces for external dependencies.
//
// This package follows the Dependency Inversion Principle by defining
// interfaces that the application layer depends on, while concrete
// implementations live in the adapter layer.
//
// # Architecture
//
// The ports package sits at the boundary between the application layer
// and the infrastructure/adapter layer:
//
//	┌─────────────────────────────────────┐
//	│         Application Layer           │
//	│  (services, use cases)              │
//	│                                     │
//	│  depends on ↓                       │
//	├─────────────────────────────────────┤
//	│         Ports (this package)        │
//	│  (interfaces only)                  │
//	│                                     │
//	│  implemented by ↓                   │
//	├─────────────────────────────────────┤
//	│         Adapter Layer               │
//	│  (concrete implementations)         │
//	│  - accessibility adapter            │
//	│  - overlay adapter                  │
//	│  - config adapter                   │
//	└─────────────────────────────────────┘
//
// # Design Principles
//
//   - Interface Segregation: Small, focused interfaces
//   - Dependency Inversion: Application depends on abstractions
//   - Context-Aware: All operations accept context for cancellation
//   - Error Handling: All operations return errors for proper handling
//
// # Usage
//
//	type MyService struct {
//		accessibility ports.AccessibilityPort
//		overlay       ports.OverlayPort
//	}
//
//	func NewMyService(
//		accessibility ports.AccessibilityPort,
//		overlay ports.OverlayPort,
//	) *MyService {
//		return &MyService{
//			accessibility: accessibility,
//			overlay:       overlay,
//		}
//	}
package ports
