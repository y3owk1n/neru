package domain

import (
	"github.com/y3owk1n/neru/internal/config"
	"go.uber.org/zap"
)

// Router provides common routing functionality for mode-based key handling.
// It can be embedded by specific routers (grid, hint) to share common logic.
type Router struct {
	Logger       *zap.Logger
	ModeExitKeys []string // Keys that exit the mode
}

// NewRouter creates a new base router with the specified logger.
func NewRouter(logger *zap.Logger) *Router {
	return &Router{
		Logger:       logger,
		ModeExitKeys: []string{},
	}
}

// NewRouterWithExitKeys creates a new base router with custom exit keys.
func NewRouterWithExitKeys(logger *zap.Logger, exitKeys []string) *Router {
	return &Router{
		Logger:       logger,
		ModeExitKeys: exitKeys,
	}
}

// IsExitKey checks if the given key is an exit key.
// Returns true if the key should trigger mode exit.
func (r *Router) IsExitKey(key string) bool {
	exitKeys := r.ModeExitKeys
	if len(exitKeys) == 0 {
		// Default to domain constant when no exit keys configured.
		// In practice, handlers always pass configured exit keys, so this is a safety fallback.
		exitKeys = []string{DefaultExitKey}
	}

	return config.IsExitKey(key, exitKeys)
}
