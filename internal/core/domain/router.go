package domain

import "go.uber.org/zap"

// Router provides common routing functionality for mode-based key handling.
type Router struct {
	Logger *zap.Logger
}

// NewRouter creates a new base router with the specified logger.
func NewRouter(logger *zap.Logger) *Router {
	return &Router{
		Logger: logger,
	}
}
