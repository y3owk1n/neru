package ipc

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

var traceIDKey = contextKey{}

// TraceID is a unique identifier stamped on every accepted IPC command so
// the daemon's logs can correlate a CLI invocation with the handler work it
// triggered. IDs travel via context; nothing else is stored.
type TraceID string

// NewTraceID generates a new unique trace ID.
func NewTraceID() TraceID {
	return TraceID(uuid.New().String())
}

// WithTraceID returns a new context with the given trace ID.
func WithTraceID(ctx context.Context, id TraceID) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// TraceIDFromContext retrieves the trace ID from the context.
// If no trace ID is present, it returns an empty string.
func TraceIDFromContext(ctx context.Context) TraceID {
	id, ok := ctx.Value(traceIDKey).(TraceID)
	if !ok {
		return ""
	}

	return id
}

// String returns the string representation of the trace ID.
func (id TraceID) String() string {
	return string(id)
}
