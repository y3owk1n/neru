package grid

import (
	"image"

	"go.uber.org/zap"
)

// Router handles key routing for grid mode operations.
type Router struct {
	manager *Manager
	logger  *zap.Logger
}

// KeyResult captures the results of key routing decisions in grid mode.
type KeyResult struct {
	exit        bool        // Escape pressed -> exit mode
	targetPoint image.Point // Complete coordinate entered
	complete    bool        // Coordinate selection complete
}

// Exit returns whether to exit grid mode.
func (kr *KeyResult) Exit() bool {
	return kr.exit
}

// TargetPoint returns the target point for the complete coordinate.
func (kr *KeyResult) TargetPoint() image.Point {
	return kr.targetPoint
}

// Complete returns whether coordinate selection is complete.
func (kr *KeyResult) Complete() bool {
	return kr.complete
}

// NewRouter initializes a new grid router with the specified manager and logger.
func NewRouter(m *Manager, logger *zap.Logger) *Router {
	return &Router{
		manager: m,
		logger:  logger,
	}
}

// RouteKey processes a keypress and determines the appropriate action in grid mode.
func (r *Router) RouteKey(key string) KeyResult {
	var routeKeyResult KeyResult

	// Exit grid mode with Escape
	if key == "\x1b" || key == "escape" {
		routeKeyResult.exit = true

		return routeKeyResult
	}

	// Delegate coordinate input to the grid manager
	if point, complete := r.manager.HandleInput(key); complete {
		r.logger.Debug("Grid router: Coordinate selection complete",
			zap.Int("x", point.X),
			zap.Int("y", point.Y))
		routeKeyResult.targetPoint = point
		routeKeyResult.complete = true
	}

	return routeKeyResult
}
