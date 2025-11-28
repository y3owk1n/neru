package ports

import "context"

// Tracer provides tracing functionality.
type Tracer interface {
	// NewID generates a new trace ID.
	NewID() string

	// WithTraceID adds a trace ID to the context.
	WithTraceID(ctx context.Context, traceID string) context.Context

	// FromContext extracts the trace ID from the context.
	FromContext(ctx context.Context) string
}
