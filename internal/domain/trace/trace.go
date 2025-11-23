// Package trace provides tracing utilities.
package trace

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

var traceIDKey = contextKey{}

// ID represents a unique trace identifier.
type ID string

// NewID generates a new unique trace ID.
func NewID() ID {
	return ID(uuid.New().String())
}

// WithTraceID returns a new context with the given trace ID.
func WithTraceID(ctx context.Context, id ID) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// FromContext retrieves the trace ID from the context.
// If no trace ID is present, it returns an empty string.
func FromContext(ctx context.Context) ID {
	id, ok := ctx.Value(traceIDKey).(ID)
	if !ok {
		return ""
	}
	return id
}

// String returns the string representation of the trace ID.
func (id ID) String() string {
	return string(id)
}
