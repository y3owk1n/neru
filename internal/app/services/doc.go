// Package services implements application use cases and orchestration logic.
//
// Services coordinate domain logic and infrastructure adapters. They depend on
// port interfaces rather than concrete implementations, so tests can run them
// over the mocks in internal/ports/mocks.
package services
