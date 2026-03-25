package grid

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
)

// Router handles key routing for grid mode operations.
type Router struct {
	*domain.Router

	manager *Manager
}

// KeyResult captures the results of key routing decisions in grid mode.
type KeyResult struct {
	targetPoint image.Point // Complete coordinate entered
	complete    bool        // Coordinate selection complete
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
		Router:  domain.NewRouter(logger),
		manager: m,
	}
}

// RouteKey processes a keypress and determines the appropriate action in grid mode.
func (r *Router) RouteKey(key string) KeyResult {
	var routeKeyResult KeyResult

	// Delegate coordinate input to the grid manager
	if point, complete := r.manager.HandleInput(key); complete {
		if r.Logger != nil {
			r.Logger.Debug("Grid router: Coordinate selection complete",
				zap.Int("x", point.X),
				zap.Int("y", point.Y))
		}

		routeKeyResult.targetPoint = point
		routeKeyResult.complete = true
	}

	return routeKeyResult
}
