package ports

import (
	"context"
)

// HealthCheck defines the interface for components that can report their health status.
type HealthCheck interface {
	// Health returns nil if the component is healthy, or an error if it is not.
	Health(context context.Context) error
}
