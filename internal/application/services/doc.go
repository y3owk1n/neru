// Package services implements application use cases and orchestration logic.
//
// Services coordinate between domain logic and infrastructure adapters,
// implementing the business workflows of the application. They depend on
// port interfaces rather than concrete implementations, enabling testability
// and flexibility.
//
// # Architecture
//
// Services sit in the application layer:
//
//	┌─────────────────────────────────────┐
//	│         Domain Layer                │
//	│  (pure business logic)              │
//	└─────────────────────────────────────┘
//	              ↑
//	              │ uses
//	┌─────────────────────────────────────┐
//	│    Application Layer (Services)     │
//	│  - HintService                      │
//	│  - ActionService                    │
//	│  - GridService                      │
//	└─────────────────────────────────────┘
//	              ↑
//	              │ depends on
//	┌─────────────────────────────────────┐
//	│         Ports (Interfaces)          │
//	└─────────────────────────────────────┘
//	              ↑
//	              │ implemented by
//	┌─────────────────────────────────────┐
//	│         Adapter Layer               │
//	│  (infrastructure)                   │
//	└─────────────────────────────────────┘
//
// # Design Principles
//
//   - Single Responsibility: Each service handles one cohesive set of use cases
//   - Dependency Injection: All dependencies injected via constructor
//   - Context-Aware: All operations accept context for cancellation/timeout
//   - Error Handling: Errors are wrapped with context for debugging
//   - Logging: Structured logging for observability
//
// # Usage
//
//	hintService := services.NewHintService(
//		accessibilityAdapter,
//		overlayAdapter,
//		hintGenerator,
//		logger,
//	)
//
//	if err := hintService.ShowHints(ctx, filter); err != nil {
//		return fmt.Errorf("failed to show hints: %w", err)
//	}
//
// # Testing
//
// Services are easily testable using mock implementations of ports:
//
//	mockAccessibility := &mocks.MockAccessibilityPort{
//		GetClickableElementsFunc: func(ctx context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
//			return testElements, nil
//		},
//	}
//
//	service := services.NewHintService(mockAccessibility, mockOverlay, mockGenerator, logger)
//	err := service.ShowHints(ctx, filter)
package services
